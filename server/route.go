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
func (router *router) NewRoute(
	path, applicationID, userID string,
	c chan *MessageForRoute,
	timeout time.Duration) *Route {

	route := &Route{
		router:        router,
		messagesC:     c,
		queue:         newQueue(),
		timeout:       timeout,
		Path:          protocol.Path(path),
		UserID:        userID,
		ApplicationID: applicationID,
	}
	return route
}

// Route represents a topic for subscription that has a channel to receive messages.
type Route struct {
	*router
	messagesC chan *MessageForRoute
	queue     *queue
	timeout   time.Duration

	// Indicates if the consumer go routine is running
	consuming bool
	invalid   bool
	mu        sync.RWMutex

	Path          protocol.Path
	UserID        string // UserID that subscribed or pushes messages to the router
	ApplicationID string
}

func (r *Route) String() string {
	return fmt.Sprintf("%s:%s:%s", r.Path, r.UserID, r.ApplicationID)
}

func (r *Route) equals(other *Route) bool {
	return r.Path == other.Path &&
		r.UserID == other.UserID &&
		r.ApplicationID == other.ApplicationID
}

// Close closes the route channel.
func (r *Route) Close() {
	// closing channel makes the route invalid
	r.mu.Lock()
	defer r.mu.Unlock()

	r.invalid = true
	close(r.messagesC)

	// release the queue
	r.queue = nil

	// route should also unsubscribe
	r.unsubscribe(r)
}

// MessagesC returns the route channel to receive messages. To send messages through
// a route use the `Deliver` method
func (r *Route) MessagesC() <-chan *MessageForRoute {
	return r.messagesC
}

// Deliver takes a messages and adds it to the queue to be delivered in to the
// channel
func (r *Route) Deliver(m *protocol.Message) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.invalid {
		return ErrInvalidRoute
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
	// no timeout, means we dont close the channel
	mr := &MessageForRoute{Message: m, Route: r}
	if r.timeout == -1 {
		r.messagesC <- mr
		return nil
	}

	select {
	case r.messagesC <- mr:
		return nil
	case <-time.After(r.timeout):
		r.Close()
		return errTimeout
	}
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
