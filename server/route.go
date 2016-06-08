package server

import (
	"errors"
	"fmt"
	"github.com/smancke/guble/protocol"
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
func (r *Route) IsInvalid() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.invalid
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
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.invalid {
		return ErrInvalidRoute
	}

	// if size is zero the sending is direct
	if r.queueSize == 0 {
		return r.sendDirect(m)
	}

	if r.queue.Len() >= r.queueSize {
		return r.Close()
	}

	r.queue.Push(m)
	if !r.consuming {
		go r.consume()
	}
	return nil
}

// consume starts a goroutine to consume the queue and pass the messages to route
// channel. The go routine stops if there are no items in the queue.
func (r *Route) consume() {
	protocol.Debug("Consuming route %s queue", r)
	defer func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.consuming = false
	}()

	var (
		m   *protocol.Message
		err error
	)

	r.mu.Lock()
	r.consuming = true
	r.mu.Unlock()

	for {
		m, err = r.queue.Pop()

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
			if err == errTimeout {
				r.Close()
				// channel been closed, ending the consumer
				return
			}
		}
	}
}

func (r *Route) send(m *protocol.Message) error {
	protocol.Debug("Sending message %s through route %s channel", m, r)

	// no timeout, means we dont close the channel
	mr := &MessageForRoute{Message: m, Route: r}
	if r.timeout == -1 {
		r.messagesC <- mr
		return nil
	}

	select {
	case r.messagesC <- mr:
		return nil
	case <-r.closeC:
		return ErrInvalidRoute
	case <-time.After(r.timeout):
		r.Close()
		return errTimeout
	}
}

// sendDirect sends the message directly in the channel
func (r *Route) sendDirect(m *protocol.Message) error {
	mr := &MessageForRoute{Message: m, Route: r}
	select {
	case r.messagesC <- mr:
		return nil
	default:
		r.Close()
		return ErrChannelFull
	}
}

// Close closes the route channel.
func (r *Route) Close() error {
	protocol.Debug("Closing route: %s", r)

	// closing channel makes the route invalid
	r.mu.RLock()
	defer r.mu.RUnlock()

	// route already closed
	if r.invalid {
		return ErrInvalidRoute
	}

	r.invalid = true
	close(r.messagesC)
	close(r.closeC)

	// release the queue
	r.queue = nil

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

func (q *queue) Push(m *protocol.Message) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.queue = append(q.queue, m)
}

func (q *queue) Pop() (*protocol.Message, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.queue) == 0 {
		return nil, errEmptyQueue
	}

	m := q.queue[0]
	q.queue = q.queue[1:]
	return m, nil
}

func (q *queue) Len() int {
	return len(q.queue)
}
