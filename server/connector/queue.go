package connector

import (
	log "github.com/Sirupsen/logrus"
	"sync"
)

// Queue is an interface modeling a task-queue (it is started and more Requests can be pushed to it, an finally it is stopped).
type Queue interface {
	ResponseHandleSetter

	Start() error
	Push(request Request) error
	Stop() error
}

type queue struct {
	sender          Sender
	responseHandler ResponseHandler
	requestsC       chan Request
	nWorkers        int
	wg              sync.WaitGroup
}

// NewQueue returns a new Queue (not started).
func NewQueue(sender Sender, nWorkers int) Queue {
	q := &queue{
		sender:    sender,
		requestsC: make(chan Request),
		nWorkers:  nWorkers,
	}
	return q
}

func (q *queue) SetResponseHandler(rh ResponseHandler) {
	q.responseHandler = rh
}

func (q *queue) ResponseHandler() ResponseHandler {
	return q.responseHandler
}

func (q *queue) Start() error {
	for i := 1; i <= q.nWorkers; i++ {
		go q.worker(i)
	}
	return nil
}

func (q *queue) worker(i int) {
	logger.WithField("worker", i).Debug("starting queue worker")
	for request := range q.requestsC {
		q.wg.Add(1)
		response, err := q.sender.Send(request)
		if q.responseHandler != nil {
			err = q.responseHandler.HandleResponse(request, response, err)
			if err != nil {
				logger.WithFields(log.Fields{
					"error":      err.Error(),
					"subscriber": request.Subscriber(),
					"message":    request.Message(),
				}).Error("Error handling connector response")
			}
		}
		q.wg.Done()
	}
}

func (q *queue) Push(request Request) error {
	q.requestsC <- request
	return nil
}

func (q *queue) Stop() error {
	close(q.requestsC)
	q.wg.Wait()
	return nil
}
