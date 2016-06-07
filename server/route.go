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
)

// NewRoute creates a new route pointer
func NewRoute(path, applicationID, userID string, chanSize, queueSize, timeout time.Duration) *Route {
	return &Route{
		Path:          protocol.Path(path),
		UserID:        userID,
		ApplicationID: applicationID,

		messagesC: make(chan *MessageForRoute, chanSize),
		queue:     newQueue(queueSize),
		timeout:   timeout,
	}
}

// Route represents a topic for subscription that has a channel to receive messages.
type Route struct {
	Path          protocol.Path
	UserID        string // UserID that subscribed or pushes messages to the router
	ApplicationID string

	messagesC chan *MessageForRoute
	queue     *queue
	timeout   time.Duration

	// Indicates if the consumer go routine is running
	consuming bool
	invalid   bool
	mu        sync.RWMutex
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
	close(r.messagesC)

	// release the queue
	r.queue = nil
}

// MessagesC returns the route channel to receive messages. To send messages through
// a route use the `Deliver` method
func (r *Route) MessagesC() <-chan *MessageForRoute {
	return r.messagesC
}

// Deliver takes a messages and adds it to the queue to be delivered in to the
// channel
func (r *Route) Deliver(m *MessageForRoute) error {
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
		m   *MessageForRoute
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

				// channel been closed, ending the consuming
				r.mu.Lock()
				r.invalid = true
				r.mu.Unlock()

				return
			}
		}
	}
}

func (r *Route) send(m *MessageForRoute) error {
	// no timeout, means we dont close the channel
	if r.timeout == -1 {
		r.messagesC <- m
		return nil
	}

	select {
	case r.messagesC <- m:
		return nil
	case <-time.After(r.timeout):
		return errTimeout
	}
}

func newQueue(size int) *queue {
	var length = 1
	if size > 0 {
		length = size
	}

	return &queue{
		size:  size,
		queue: make([]*MessageForRoute, 0, length),
	}
}

type queue struct {
	mu    sync.Mutex
	size  int
	queue []*MessageForRoute
}

func (q *queue) Push(m *MessageForRoute) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.queue = append(q.queue, m)
}

func (q *queue) Pop() (*MessageForRoute, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.queue) == 0 {
		return nil, errEmptyQueue
	}

	m := q.queue[0]
	q.queue = q.queue[1:]
	return m, nil
}
