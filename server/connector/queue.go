package connector

import (
	"strconv"
	"sync"

	log "github.com/Sirupsen/logrus"
)

const QueueSize = 5000

type Queue interface {
	Push(request Request) error
	Close()
}

type queue struct {
	sender   Sender
	handler  ResponseHandler
	requests chan Request
	wg       sync.WaitGroup
	nWorkers int
}

func NewQueue(sender Sender, handler ResponseHandler, nWorkers int) Queue {
	q := &queue{
		sender:   sender,
		handler:  handler,
		requests: make(chan Request, QueueSize),
		nWorkers: nWorkers,
	}
	q.start()
	return q
}

func (q *queue) start() {
	for i := 1; i < q.nWorkers; i++ {
		go q.worker(strconv.Itoa(i))
	}
}

func (q *queue) Push(request Request) error {
	q.requests <- request
	return nil
}

func (q *queue) worker(name string) {
	for request := range q.requests {
		q.wg.Add(1)
		response, err := q.sender.Send(request)
		err = q.handler.HandleResponse(request.Subscriber(), response, err)
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

}
