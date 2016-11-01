package connector

import (
	"sync"

	log "github.com/Sirupsen/logrus"
)

type Queue interface {
	Start() error
	Push(request Request) error
	Close() error
}

type queue struct {
	sender    Sender
	handler   ResponseHandler
	requestsC chan Request
	nWorkers  int
	wg        sync.WaitGroup
}

func NewQueue(sender Sender, handler ResponseHandler, nWorkers int) Queue {
	q := &queue{
		sender:    sender,
		handler:   handler,
		requestsC: make(chan Request),
		nWorkers:  nWorkers,
	}
	return q
}

func (q *queue) Start() error {
	for i := 1; i < q.nWorkers; i++ {
		go q.worker()
	}
	return nil
}

func (q *queue) worker() {
	for request := range q.requestsC {
		q.wg.Add(1)
		response, err := q.sender.Send(request)
		err = q.handler.HandleResponse(request, response, err)
		if err != nil {
			log.WithFields(log.Fields{
				"error":      err.Error(),
				"subscriber": request.Subscriber(),
				"message":    request.Message(),
			}).Error("Error handling connector response")
		}
		q.wg.Done()
	}
}

func (q *queue) Push(request Request) error {
	q.requestsC <- request
	return nil
}

func (q *queue) Close() error {
	close(q.requestsC)
	q.wg.Wait()
	return nil
}
