package server

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/smancke/guble/protocol"

	log "github.com/Sirupsen/logrus"
)

const (
	defaultQueueCap = 50
)

var (
	errEmptyQueue = errors.New("Empty queue")
	errTimeout    = errors.New("Channel sending timeout")
)

type RouteOptions struct {
	Path protocol.Path

	// channel buffer size
	Size int

	// QueueSize
	QueueSize int

	// Timeout to define how long to wait for the message to be read on the channel
	// if timeout is reached the route is closed
	Timeout time.Duration
}

type RouteParams map[string]string

func (rp *RouteParams) String() string {
	s := make([]string, 0, len(*rp))
	for k, v := range *rp {
		s = append(s, fmt.Sprintf("%s: %s", k, v))
	}
	return strings.Join(s, " ")
}

func (rp *RouteParams) Equal(other RouteParams) bool {
	if len(*rp) != len(other) {
		return false
	}

	for k, v := range *rp {
		if v2, ok := other[k]; !ok {
			return false
		} else if v != v2 {
			return false
		}
	}

	return true
}

func (rp *RouteParams) Get(key string) string {
	return (*rp)[key]
}

func (rp *RouteParams) Set(key, value string) {
	(*rp)[key] = value
}

// Route represents a topic for subscription that has a channel to receive messages.
type Route struct {
	Options RouteOptions
	Params  RouteParams

	messagesC chan *protocol.Message

	// queue that will store the messages in correct order
	//
	// The queue can have a settable size and if it reaches the capacity the
	// route is closed
	queue *queue

	closeC chan struct{}

	// Indicates if the consumer go routine is running
	consuming bool
	invalid   bool
	mu        sync.RWMutex

	logger *log.Entry
}

// NewRoute creates a new route pointer
// 	- `size` is the channel buffer size
func NewRoute(path, applicationID, userID string, size int) *Route {
	options := RouteOptions{
		Path: protocol.Path(path),
		Size: size,
	}
	params := RouteParams{
		"applicationID": applicationID,
		"userID":        userID,
	}

	route := &Route{
		Options: options,
		Params:  params,

		queue:     newQueue(options.QueueSize),
		messagesC: make(chan *protocol.Message, size),
		closeC:    make(chan struct{}),

		logger: logger.WithFields(log.Fields{"path": path, "params": params}),
	}
	return route
}

// SetTimeout sets the timeout duration that the route will wait before it will close a
// blocking channel
func (r *Route) SetTimeout(timeout time.Duration) *Route {
	r.Options.Timeout = timeout
	return r
}

// SetQueueSize sets the queue size that is allowed before closing the route channel
func (r *Route) SetQueueSize(size int) *Route {
	r.Options.QueueSize = size
	return r
}

func (r *Route) String() string {
	return fmt.Sprintf("Path: %s %s", r.Options.Path, r.Params)
}

// Deliver takes a messages and adds it to the queue to be delivered in to the
// channel
func (r *Route) Deliver(msg *protocol.Message) error {
	logger := r.logger.WithField("message", msg)
	logger.Debug("Delivering message")

	if r.isInvalid() {
		logger.Debug("Cannot deliver because route is invalid")
		mTotalDeliverMessageErrors.Add(1)
		return ErrInvalidRoute
	}

	// not an infinite queue
	if r.Options.QueueSize >= 0 {
		// if size is zero the sending is direct
		if r.Options.QueueSize == 0 {
			return r.sendDirect(msg)
		} else if r.queue.size() >= r.Options.QueueSize {
			logger.Debug("Closing route because queue is full")
			r.Close()
			mTotalDeliverMessageErrors.Add(1)
			return ErrQueueFull
		}
	}

	r.queue.push(msg)
	logger.WithField("size", r.queue.size()).Debug("Queue size")

	r.consume()
	return nil
}

// MessagesChannel returns the route channel to send or receive messages.
func (r *Route) MessagesChannel() <-chan *protocol.Message {
	return r.messagesC
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

func (r *Route) equals(other *Route) bool {
	return r.Options.Path == other.Options.Path && r.Params.Equal(other.Params)
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
	if r.Options.Timeout == -1 {
		r.messagesC <- msg
		r.logger.WithField("size", len(r.messagesC)).Debug("Channel size")
		return nil
	}

	select {
	case r.messagesC <- msg:
		return nil
	case <-r.closeC:
		return ErrInvalidRoute
	case <-time.After(r.Options.Timeout):
		r.logger.Debug("Closing route because of timeout")
		r.Close()
		return errTimeout
	}
}

// invalidRecover is used to recoverd in case we end up sending on a closed channel
func (r *Route) invalidRecover() error {
	if rc := recover(); rc != nil && r.isInvalid() {
		r.logger.WithField("error", rc).Debug("Recovered closed router")
		return ErrInvalidRoute
	}
	return nil
}

// sendDirect sends the message directly in the channel
func (r *Route) sendDirect(msg *protocol.Message) error {
	select {
	case r.messagesC <- msg:
		return nil
	default:
		r.logger.Debug("Closing route because of full channel")
		r.Close()
		return ErrChannelFull
	}
}
