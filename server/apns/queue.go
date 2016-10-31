package apns

import (
	"github.com/sideshow/apns2"
	"github.com/smancke/guble/protocol"
	"sync"
)

type queue struct {
	client     Pusher
	requestsC  chan *fullRequest
	responsesC chan *fullResponse
	wg         sync.WaitGroup
}

// fullRequest after sending a notification.
type fullRequest struct {
	notification *apns2.Notification
	message      *protocol.Message
	sub          *sub
}

// fullResponse after sending a notification.
type fullResponse struct {
	fullRequest *fullRequest
	response    *apns2.Response
	err         error
}

// NewQueue returns a pointer to a Queue using a client and a fixed number of workers / goroutines.
func NewQueue(client Pusher, nWorkers uint) *queue {
	// unbuffered channels
	q := &queue{
		client:     client,
		requestsC:  make(chan *fullRequest),
		responsesC: make(chan *fullResponse),
	}
	for i := uint(0); i < nWorkers; i++ {
		go worker(q)
	}
	return q
}

// Push queues a notification to the APNS.
func (q *queue) push(n *apns2.Notification, m *protocol.Message, s *sub) {
	q.requestsC <- &fullRequest{n, m, s}
}

// Close the channels for notifications and responses and shutdown workers.
// Waits until all responses are consumed from the channel.
func (q *queue) Close() {
	close(q.requestsC)
	q.wg.Wait()
	close(q.responsesC)
}

func worker(q *queue) {
	for fullReq := range q.requestsC {
		q.wg.Add(1)
		response, err := q.client.Push(fullReq.notification)
		q.responsesC <- &fullResponse{
			fullRequest: fullReq,
			response:    response,
			err:         err,
		}
		q.wg.Done()
	}
}
