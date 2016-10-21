package apns

import (
	"github.com/sideshow/apns2"
)

type Queue struct {
	client         Pusher
	notificationsC chan *apns2.Notification
	responsesC     chan *FullResponse
}

// FullResponse after sending a notification.
type FullResponse struct {
	Notification *apns2.Notification
	Response     *apns2.Response
	Err          error
}

// NewQueue returns a pointer to a Queue using a client and a fixed number of workers / goroutines.
func NewQueue(client Pusher, nWorkers uint) *Queue {
	// unbuffered channels
	q := &Queue{
		client:         client,
		notificationsC: make(chan *apns2.Notification),
		responsesC:     make(chan *FullResponse),
	}
	for i := uint(0); i < nWorkers; i++ {
		go worker(q)
	}
	return q
}

// Push queues a notification to the APNS.
func (q *Queue) Push(n *apns2.Notification) {
	q.notificationsC <- n
}

// Close the channels for notifications and responses and shutdown workers. Should be called after all responses have been received.
func (q *Queue) Close() {
	// Stop accepting new notifications and shutdown workers after existing notifications are processed
	close(q.notificationsC)
	// Close responses channel
	close(q.responsesC)
}

func worker(q *Queue) {
	for n := range q.notificationsC {
		response, err := q.client.Push(n)
		q.responsesC <- &FullResponse{
			Notification: n,
			Response:     response,
			Err:          err,
		}
	}
}
