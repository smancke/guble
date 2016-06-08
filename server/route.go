package server

import (
	"errors"
	"fmt"
	"github.com/smancke/guble/protocol"
	"runtime"
	"sync"
	"time"
)

var (
	errEmptyQueue = errors.New("Empty queue")
	errTimeout    = errors.New("Channel sending timeout")

	defaultQueueCap = 50
)

// NewRoute creates a new route pointer
func NewRoute(path, applicationID, userID string, c chan *MessageForRoute) *Route {
	route := &Route{
		messagesC:     c,
		queue:         newQueue(),
		queueSize:     0,
		timeout:       -1,
		closeC:        make(chan bool),
		Path:          protocol.Path(path),
		UserID:        userID,
		ApplicationID: applicationID,
	}
	return route
}

// Route represents a topic for subscription that has a channel to receive messages.
type Route struct {
	messagesC chan *MessageForRoute
	queue     *queue
	queueSize int           // Allowed queue size
	timeout   time.Duration // timeout before closing channel
	closeC    chan bool

	// Indicates if the consumer go routine is running
	consuming bool
	invalid   bool
	mu        sync.RWMutex

	Path          protocol.Path
	UserID        string // UserID that subscribed or pushes messages to the router
	ApplicationID string
}

// SetTimeout sets the timeout duration that the route will wait before it will close a
// blocking channel
func (r *Route) SetTimeout(timeout time.Duration) *Route {
	r.timeout = timeout
	return r
}

// SetQueueSize sets the queue size that is allowed before closing the route channel
func (r *Route) SetQueueSize(size int) *Route {
	r.queueSize = size
	return r
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

func (r *Route) String() string {
	return fmt.Sprintf("%s:%s:%s", r.Path, r.UserID, r.ApplicationID)
}

func (r *Route) equals(other *Route) bool {
	return r.Path == other.Path &&
		r.UserID == other.UserID &&
		r.ApplicationID == other.ApplicationID
}

// MessagesC returns the route channel to receive messages. To send messages through
// a route use the `Deliver` method
func (r *Route) MessagesC() chan *MessageForRoute {
	return r.messagesC
}

// Deliver takes a messages and adds it to the queue to be delivered in to the
// channel
func (r *Route) Deliver(m *protocol.Message) error {
	protocol.Debug("Delivering message %s", m)

	if r.isInvalid() {
		protocol.Debug("Cannot deliver cause route is invalid")
		return ErrInvalidRoute
	}

	// if size is zero the sending is direct
	if r.queueSize > -1 {
		// not an infinite queue
		if r.queueSize == 0 {
			return r.sendDirect(m)
		} else if r.queue.len() >= r.queueSize {
			protocol.Debug("Closing route cause of full queue")
			r.Close()
			return ErrQueueFull
		}
	}

	r.queue.push(m)
	protocol.Debug("Queue size: %d", r.queue.len())

	if !r.isConsuming() {
		protocol.Debug("Starting consuming")
		go r.consume()
	}

	runtime.Gosched()
	return nil
}

// consume starts a goroutine to consume the queue and pass the messages to route
// channel. The go routine stops if there are no items in the queue.
func (r *Route) consume() {
	protocol.Debug("Consuming route %s queue", r)
	r.setConsuming(true)
	defer r.setConsuming(false)

	var (
		m   *protocol.Message
		err error
	)

	for {
		if r.isInvalid() {
			protocol.Debug("Stopping consuming cause of invalid route.")
			return
		}

		m, err = r.queue.get()

		if err != nil {
			if err == errEmptyQueue {
				protocol.Debug("Empty queue")
				return
			}
			protocol.Err("Error fetching queue message %s", err)
			continue
		}

		// send next message throught the channel
		if err = r.send(m); err != nil {
			protocol.Err("Error sending message %s through route %s", m, r)
			if err == errTimeout || err == ErrInvalidRoute {
				// channel been closed, ending the consumer
				return
			}
		}
		// all good shift the first item out of the queue
		r.queue.shift()
	}
}

func (r *Route) send(m *protocol.Message) error {
	defer r.invalidRecover()
	protocol.Debug("Sending message %s through route %s channel", m, r)

	// no timeout, means we dont close the channel
	if r.timeout == -1 {
		r.messagesC <- r.format(m)
		protocol.Debug("Channel size: %d", len(r.messagesC))
		return nil
	}

	select {
	case r.messagesC <- r.format(m):
		return nil
	case <-r.closeC:
		return ErrInvalidRoute
	case <-time.After(r.timeout):
		protocol.Debug("Closing route cause of timeout")
		r.Close()
		return errTimeout
	}
}

// recover function to use when sending on closed channel
func (r *Route) invalidRecover() error {
	if rc := recover(); rc != nil && r.isInvalid() {
		protocol.Debug("Recovered closed router err: %s", rc)
		return ErrInvalidRoute
	}
	return nil
}

// format message according to the channel format
// TODO: this will be dropped cause it's used only in GCM and after implementing
// separated subscriptions wont be required anymore
func (r *Route) format(m *protocol.Message) *MessageForRoute {
	return &MessageForRoute{Message: m, Route: r}
}

// sendDirect sends the message directly in the channel
func (r *Route) sendDirect(m *protocol.Message) error {
	mr := &MessageForRoute{Message: m, Route: r}
	select {
	case r.messagesC <- mr:
		return nil
	default:
		protocol.Debug("Closing route cause of fullchannel")
		r.Close()
		return ErrChannelFull
	}
}

// Close closes the route channel.
func (r *Route) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	protocol.Debug("Closing route: %s", r)

	// route already closed
	if r.invalid {
		return ErrInvalidRoute
	}

	r.invalid = true
	close(r.messagesC)
	close(r.closeC)

	return ErrInvalidRoute
}

func newQueue() *queue {
	return &queue{
		queue: make([]*protocol.Message, 0, defaultQueueCap),
	}
}

type queue struct {
	mu    sync.Mutex
	queue []*protocol.Message
}

func (q *queue) push(m *protocol.Message) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.queue = append(q.queue, m)
}

// Remove the first item from the queue if exists
func (q *queue) shift() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) == 0 {
		return
	}
	q.queue = q.queue[1:]
}

// return the first item from the queue without removing it
func (q *queue) get() (*protocol.Message, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) == 0 {
		return nil, errEmptyQueue
	}

	return q.queue[0], nil
}

func (q *queue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queue)
}
