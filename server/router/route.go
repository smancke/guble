package router

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/store"
)

var (
	errEmptyQueue = errors.New("Empty queue")
	errTimeout    = errors.New("Channel sending timeout")

	ErrMissingFetchRequest = errors.New("Missing FetchRequest configuration.")
)

// Route represents a topic for subscription that has a channel to receive messages.
type Route struct {
	RouteConfig

	messagesC chan *protocol.Message

	// queue that will store the messages in correct order.
	// The queue can have a settable size;
	// if it reaches the capacity the route is closed.
	queue *queue

	closeC chan struct{}

	// Indicates if the consumer go routine is running
	consuming bool
	invalid   bool
	mu        sync.RWMutex

	logger *log.Entry
}

// NewRoute creates a new route pointer
func NewRoute(config RouteConfig) *Route {
	route := &Route{
		RouteConfig: config,

		queue:     newQueue(config.QueueSize),
		messagesC: make(chan *protocol.Message, config.ChannelSize),
		closeC:    make(chan struct{}),

		logger: logger.WithFields(log.Fields{"path": config.Path, "params": config.RouteParams}),
	}

	return route
}

// Key returns a string that uniquely identifies the route
// by concatenating the route Path and the route params
// Example:
//  /topic user_id:user1 application_id:app1
func (r *Route) Key() string {
	return strings.Join([]string{
		string(r.Path),
		r.RouteParams.Key(),
	}, " ")
}

func (r *Route) String() string {
	return fmt.Sprintf("Path: %s , Params: %s", r.Path, r.RouteParams)
}

// Deliver takes a messages and adds it to the queue to be delivered into the channel
// isFromStore boolean specifies if the messages are being fetched or are from the router
// In case they are fetched from the store the route won't close if it's full
func (r *Route) Deliver(msg *protocol.Message, isFromStore bool) error {
	loggerMessage := r.logger.WithField("message", msg)

	if r.isInvalid() {
		loggerMessage.Error("Cannot deliver because route is invalid")
		mTotalDeliverMessageErrors.Add(1)
		return ErrInvalidRoute
	}

	if !r.messageFilter(msg) {
		loggerMessage.Debug("Message filter didn't match route")
		mTotalNotMatchedByFilters.Add(1)
		return nil
	}
	// not an infinite queue
	if r.QueueSize >= 0 {
		// if size is zero the sending is direct
		if r.QueueSize == 0 {
			return r.sendDirect(msg, isFromStore)
		} else if r.queue.size() >= r.QueueSize {
			loggerMessage.Error("Closing route because queue is full")
			r.Close()
			mTotalDeliverMessageErrors.Add(1)
			return ErrQueueFull
		}
	}

	r.queue.push(msg)
	loggerMessage.WithField("queue_size", r.queue.size()).Debug("Deliver")

	r.consume()
	return nil
}

// MessagesChannel returns the route channel to send or receive messages.
func (r *Route) MessagesChannel() <-chan *protocol.Message {
	return r.messagesC
}

// Provide accepts a router to use for fetching/subscribing and a boolean
// indicating if it should close the route after fetching without subscribing
// The method is blocking until fetch is finished or route is subscribed
func (r *Route) Provide(router Router, subscribe bool) error {
	if r.FetchRequest != nil {
		err := r.handleFetch(router)
		if err != nil {
			return err
		}
	} else if !subscribe {
		return ErrMissingFetchRequest
	}
	if !subscribe {
		return nil
	}
	return r.handleSubscribe(router)
}

func (r *Route) handleFetch(router Router) error {
	if r.isInvalid() {
		return ErrInvalidRoute
	}

	r.FetchRequest.Partition = r.Path.Partition()
	ms, err := router.MessageStore()
	if err != nil {
		return err
	}

	var (
		lastID   uint64
		received int
	)

REFETCH:
	// check if we need to continue fetching
	maxID, err := ms.MaxMessageID(r.FetchRequest.Partition)
	if err != nil {
		return err
	}

	if r.FetchRequest.StartID > maxID && r.FetchRequest.Direction == store.DirectionForward {
		return nil
	}

	if received >= r.FetchRequest.Count || lastID >= maxID ||
		(r.FetchRequest.EndID > 0 && r.FetchRequest.EndID <= lastID) {
		return nil
	}
	r.FetchRequest.Init()

	if err := router.Fetch(r.FetchRequest); err != nil {
		return err
	}
	count := r.FetchRequest.Ready()
	r.logger.WithField("count", count).Debug("Receiving messages")

	for {
		select {
		case fetchedMessage, open := <-r.FetchRequest.Messages():
			if !open {
				r.logger.Debug("Fetch channel closed.")
				goto REFETCH
			}

			r.logger.WithField("fetchedMessageID", fetchedMessage.ID).Debug("Fetched message")
			message, err := protocol.ParseMessage(fetchedMessage.Message)
			if err != nil {
				return err
			}

			r.logger.WithField("messageID", message.ID).Debug("Sending fetched message in channel")
			if err := r.Deliver(message, true); err != nil {
				return err
			}
			lastID = message.ID
			received++
		case err := <-r.FetchRequest.Errors():
			return err
		case <-router.Done():
			r.logger.Debug("Stopping fetch because the router is shutting down")
			return nil
		}
	}
}

func (r *Route) handleSubscribe(router Router) error {
	_, err := router.Subscribe(r)
	return err
}

// Close closes the route channel.
func (r *Route) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logger.Debug("Closing route")

	// route already closed
	if r.invalid {
		return ErrInvalidRoute
	}

	r.invalid = true
	close(r.messagesC)
	close(r.closeC)

	return ErrInvalidRoute
}

// Equal will check if the route path is matched and all the parameters or just a
// subset of specific parameters between the routes
func (r *Route) Equal(other *Route, keys ...string) bool {
	return r.RouteConfig.Equal(other.RouteConfig, keys...)
}

// IsInvalid returns true if the route is invalid, has been closed previously
func (r *Route) isInvalid() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.invalid
}

func (r *Route) setInvalid(invalid bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.invalid = invalid
}

func (r *Route) isConsuming() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.consuming
}

func (r *Route) setConsuming(consuming bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.consuming = consuming
}

// consume starts a goroutine to consume the queue and pass the messages to route
// channel. Stops if there are no items in the queue.
func (r *Route) consume() {
	if r.isConsuming() {
		return
	}
	r.setConsuming(true)

	r.logger.Debug("Consuming route queue")
	go func() {
		defer r.setConsuming(false)

		var (
			msg *protocol.Message
			err error
		)

		for {
			if r.isInvalid() {
				r.logger.Debug("Stopping to consume because route is invalid.")
				mTotalDeliverMessageErrors.Add(1)
				return
			}

			msg, err = r.queue.poll()

			if err != nil {
				if err == errEmptyQueue {
					r.logger.Debug("Empty queue")
					return
				}
				r.logger.WithField("error", err).Error("Error fetching a message from queue")
				continue
			}

			if err = r.send(msg); err != nil {
				r.logger.WithField("message", msg).Error("Error sending message through route")
				if err == errTimeout || err == ErrInvalidRoute {
					// channel been closed, ending the consumer
					return
				}
			}
			// remove the first item from the queue
			r.queue.remove()
		}
	}()
	runtime.Gosched()
}

// send message through the channel
func (r *Route) send(msg *protocol.Message) error {
	defer r.invalidRecover()

	r.logger.WithField("message", msg).Debug("Sending message through route channel")

	// no timeout, means we don't close the channel
	if r.timeout == -1 {
		r.messagesC <- msg
		r.logger.WithField("size", len(r.messagesC)).Debug("Channel size")
		return nil
	}

	select {
	case r.messagesC <- msg:
		return nil
	case <-r.closeC:
		return ErrInvalidRoute
	case <-time.After(r.timeout):
		r.logger.Debug("Closing route because of timeout")
		r.Close()
		return errTimeout
	}
}

// invalidRecover is used to recover in case we end up sending on a closed channel
func (r *Route) invalidRecover() error {
	if rc := recover(); rc != nil && r.isInvalid() {
		r.logger.WithField("error", rc).Debug("Recovered closed route")
		return ErrInvalidRoute
	}
	return nil
}

// sendDirect sends the message directly in the channel
func (r *Route) sendDirect(msg *protocol.Message, store bool) error {
	if store {
		r.messagesC <- msg
		return nil
	}

	select {
	case r.messagesC <- msg:
		return nil
	default:
		r.logger.Debug("Closing route because of full channel")
		r.Close()
		return ErrChannelFull
	}
}
