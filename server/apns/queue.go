package apns

import (
	"github.com/sideshow/apns2"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/connector"
	"sync"
)

type Queue struct {
	sender          connector.Sender
	responseHandler connector.ResponseHandler
	requestsC       chan connector.Request
	responsesC      chan *fullResponse
	wg              sync.WaitGroup
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
func NewQueue(sender connector.Sender, responseHandler connector.ResponseHandler, nWorkers uint) *Queue {
	// unbuffered channels
	q := &Queue{
		sender:          sender,
		responseHandler: responseHandler,
		requestsC:       make(chan connector.Request),
		responsesC:      make(chan *fullResponse),
	}
	for i := uint(0); i < nWorkers; i++ {
		go worker(q)
	}
	return q
}

// Push queues a notification to the APNS.
func (q *Queue) Push(request connector.Request) {
	q.requestsC <- request
}

// Close the channels for notifications and responses and shutdown workers.
// Waits until all responses are consumed from the channel.
func (q *Queue) Close() {
	close(q.requestsC)
	q.wg.Wait()
	close(q.responsesC)
}

func worker(q *Queue) {
	for request := range q.requestsC {
		q.wg.Add(1)
		response, err := q.sender.Send(request)
		if err != nil {
			//TODO Cosmin log + `continue` ?
		}
		q.responseHandler.HandleResponse(request.Subscriber(), response)

		//q.responsesC <- &fullResponse{
		//	fullRequest: request,
		//	response:    response,
		//	err:         err,
		//}

		q.wg.Done()
	}
}
