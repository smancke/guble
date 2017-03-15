package server

import (
	"fmt"
	"github.com/cosminrentea/gobbler/client"
	"github.com/cosminrentea/gobbler/server/service"
	"github.com/stretchr/testify/assert"
	"strconv"
	"sync"
	"testing"
	"time"
)

const (
	testTopic = "/topic"
)

type sender func(c client.Client) error

func sendMessageSample(c client.Client) error {
	return c.Send(testTopic, "test-body", "{id:id}")
}

type benchParams struct {
	*testing.B
	workers       int           // number of workers
	subscriptions int           // number of subscriptions listening on the topic
	timeout       time.Duration // timeout response
	clients       int           // number of clients
	sender        sender        // the function that will send the messages
	sent          int           // sent messages
	received      int           // received messages

	service  *service.Service
	receiveC chan bool
	doneC    chan struct{}

	wg    sync.WaitGroup
	start time.Time
	end   time.Time
}

func (params *benchParams) createClients() (clients []client.Client) {
	wsURL := "ws://" + params.service.WebServer().GetAddr() + "/stream/user/"
	for clientID := 0; clientID < params.clients; clientID++ {
		location := wsURL + strconv.Itoa(clientID)
		c, err := client.Open(location, "http://localhost/", 1000, true)
		if err != nil {
			assert.FailNow(params, "guble client could not connect to server")
		}
		clients = append(clients, c)
	}
	return
}

func (params *benchParams) receiveLoop() {
	for i := 0; i <= params.workers; i++ {
		go func() {
			for {
				select {
				case <-params.receiveC:
					params.received++
					logger.WithField("received", params.received).Debug("Received a call")
					params.wg.Done()
				case <-params.doneC:
					return
				}
			}
		}()
	}
}

func (params *benchParams) String() string {
	return fmt.Sprintf(`
		Throughput %.2f messages/second using:
			%d workers
			%d subscriptions
			%s response timeout
			%d clients
	`, params.messagesPerSecond(), params.workers, params.subscriptions, params.timeout, params.clients)
}

func (params *benchParams) ResetTimer() {
	params.start = time.Now()
	params.B.ResetTimer()
}

func (params *benchParams) StopTimer() {
	params.end = time.Now()
	params.B.StopTimer()
}

func (params *benchParams) duration() time.Duration {
	return params.end.Sub(params.start)
}

func (params *benchParams) messagesPerSecond() float64 {
	return float64(params.received) / params.duration().Seconds()
}
