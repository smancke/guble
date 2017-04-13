package router

import (
	"github.com/cosminrentea/gobbler/protocol"

	"sync"
)

const (
	defaultQueueCap = 50
)

type queue struct {
	mu    sync.Mutex
	queue []*protocol.Message
}

// newQueue creates a *queue that will have the capacity specified by size.
// If `size` is negative use the defaultQueueCap.
func newQueue(size int) *queue {
	if size < 0 {
		size = defaultQueueCap
	}
	return &queue{
		queue: make([]*protocol.Message, 0, size),
	}
}

func (q *queue) push(m *protocol.Message) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.queue = append(q.queue, m)
}

// remove the first item from the queue if exists
func (q *queue) remove() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) == 0 {
		return
	}
	q.queue = q.queue[1:]
}

// poll returns the first item from the queue without removing it
func (q *queue) poll() (*protocol.Message, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) == 0 {
		return nil, errEmptyQueue
	}

	return q.queue[0], nil
}

func (q *queue) size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queue)
}
