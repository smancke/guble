package connector

import (
	"sync"

	log "github.com/Sirupsen/logrus"
)

type Queue interface {
	Push(request Request) error
	Close()
}

type queue struct {
	sender    Sender
	handler   ResponseHandler
	requestsC chan Request
	wg        sync.WaitGroup
}

func NewQueue(sender Sender, handler ResponseHandler, nWorkers int) Queue {
	q := &queue{
		sender:    sender,
		handler:   handler,
		requestsC: make(chan Request),
	}
	q.start(nWorkers)
	return q
}

func (q *queue) start(nWorkers int) {
	for i := 1; i < nWorkers; i++ {
		go q.worker()
	}
}

func (q *queue) Push(request Request) error {
	q.requestsC <- request
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

func (q *queue) Close() {
	close(q.requestsC)
	q.wg.Wait()
}
