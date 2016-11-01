package connector

import (
	"sync"
)

type Queue interface {
	Push(request Request)
	Close()
}

type queue struct {
	sender          Sender
	responseHandler ResponseHandler
	requestsC       chan Request
	wg              sync.WaitGroup
}

// NewQueue returns a pointer to a Queue using a client and a fixed number of workers / goroutines.
func NewQueue(sender Sender, responseHandler ResponseHandler, nWorkers uint) Queue {
	// unbuffered channels
	q := &queue{
		sender:          sender,
		responseHandler: responseHandler,
		requestsC:       make(chan Request),
	}
	for i := uint(0); i < nWorkers; i++ {
		go worker(q)
	}
	return q
}

// Push queues a notification to the APNS.
func (q *queue) Push(request Request) {
	q.requestsC <- request
}

// Close the channels for notifications and responses and shutdown workers.
// Waits until all responses are consumed from the channel.
func (q *queue) Close() {
	close(q.requestsC)
	q.wg.Wait()
}

func worker(q *queue) {
	for request := range q.requestsC {
		q.wg.Add(1)
		response, errSend := q.sender.Send(request)
		q.responseHandler.HandleResponse(request, response, errSend)
		q.wg.Done()
	}
}
