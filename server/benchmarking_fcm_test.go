package server

import (
	log "github.com/Sirupsen/logrus"

	"github.com/smancke/guble/client"
	"github.com/smancke/guble/server/fcm"
	"github.com/smancke/guble/server/service"
	"github.com/smancke/guble/testutil"

	"github.com/stretchr/testify/assert"

	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	fcmTopic = "/topic"
)

// FCM benchmarks
// Default number of clients and subscriptions are 8, for tests that do not
// specify this in their name
func BenchmarkFCM_1Workers50MilliTimeout(b *testing.B) {
	params := &benchParams{
		B:             b,
		workers:       1,
		subscriptions: 8,
		timeout:       50 * time.Millisecond,
		clients:       8,
		sender:        sendMessageSample,
	}
	params.throughputSend()
	fmt.Println(params)
}

func BenchmarkFCM_8Workers50MilliTimeout(b *testing.B) {
	params := &benchParams{
		B:             b,
		workers:       8,
		subscriptions: 8,
		timeout:       50 * time.Millisecond,
		clients:       8,
		sender:        sendMessageSample,
	}
	params.throughputSend()
	fmt.Println(params)
}

func BenchmarkFCM_16Workers50MilliTimeout(b *testing.B) {
	params := &benchParams{
		B:             b,
		workers:       16,
		subscriptions: 8,
		timeout:       50 * time.Millisecond,
		clients:       8,
		sender:        sendMessageSample,
	}
	params.throughputSend()
	fmt.Println(params)
}

func BenchmarkFCM_1Workers100MilliTimeout(b *testing.B) {
	params := &benchParams{
		B:             b,
		workers:       1,
		subscriptions: 8,
		timeout:       100 * time.Millisecond,
		clients:       8,
		sender:        sendMessageSample,
	}
	params.throughputSend()
	fmt.Println(params)
}

func BenchmarkFCM_8Workers100MilliTimeout(b *testing.B) {
	params := &benchParams{
		B:             b,
		workers:       8,
		subscriptions: 8,
		timeout:       100 * time.Millisecond,
		clients:       8,
		sender:        sendMessageSample,
	}
	params.throughputSend()
	fmt.Println(params)
}

func BenchmarkFCM_16Workers100MilliTimeout(b *testing.B) {
	params := &benchParams{
		B:             b,
		workers:       16,
		subscriptions: 8,
		timeout:       100 * time.Millisecond,
		clients:       8,
		sender:        sendMessageSample,
	}
	params.throughputSend()
	fmt.Println(params)
}

func sendMessageSample(c client.Client) error {
	return c.Send(fcmTopic, "test-body", "{id:id}")
}

type sender func(c client.Client) error

type benchParams struct {
	*testing.B
	workers       int           // number of fcm workers
	subscriptions int           // number of subscriptions listening on the topic
	timeout       time.Duration // gcm timeout response
	clients       int           // number of clients
	sender        sender        // the function that will send the messages
	sent          int           // sent messages
	received      int           // received messages
	dir           string

	service  *service.Service
	receiveC chan bool
	doneC    chan struct{}

	wg    sync.WaitGroup
	start time.Time
	end   time.Time
}

func (params *benchParams) String() string {
	return fmt.Sprintf(`
		Throughput %.2f messages/second using:
			%d workers
			%d gcm subscriptions
			%s gcm response timeout
			%d clients
	`, params.mps(), params.workers, params.subscriptions, params.timeout, params.clients)
}

func (params *benchParams) expectedMessagesCount() int {
	return params.N * params.clients * params.subscriptions
}

func (params *benchParams) send(c client.Client) error {
	err := params.sender(c)
	if err != nil {
		return err
	}
	params.sent++
	return nil
}

func (params *benchParams) receiveLoop() {
	for i := 0; i <= params.workers; i++ {
		go func() {
			for {
				select {
				case <-params.receiveC:
					params.received++
					logger.WithField("received", params.received).Info("Received gcm call")
					params.wg.Done()
				case <-params.doneC:
					return
				}
			}
		}()
	}
}

// start the service
func (params *benchParams) setUp() {
	params.doneC = make(chan struct{})
	params.receiveC = make(chan bool)

	a := assert.New(params)

	dir, errTempDir := ioutil.TempDir("", "guble_benchmarking_gcm_test")
	a.NoError(errTempDir)

	params.dir = dir

	*Config.HttpListen = "localhost:0"
	*Config.KVS = "memory"
	*Config.MS = "file"
	*Config.StoragePath = dir
	*Config.FCM.Enabled = true
	*Config.FCM.APIKey = "WILL BE OVERWRITTEN"
	*Config.FCM.Workers = params.workers

	params.service = StartService()

	var fcmConn *fcm.Connector
	var ok bool
	for _, iface := range params.service.ModulesSortedByStartOrder() {
		fcmConn, ok = iface.(*fcm.Connector)
		if ok {
			break
		}
	}
	a.True(ok, "There should be a module of type GCMConnector")

	fcmConn.Sender = testutil.CreateFcmSender(
		testutil.CreateRoundTripperWithCountAndTimeout(http.StatusOK, testutil.SuccessFCMResponse, params.receiveC, params.timeout))

	urlFormat := fmt.Sprintf("http://%s/gcm/%%d/gcmId%%d/subscribe/%%s", params.service.WebServer().GetAddr())
	for i := 1; i <= params.subscriptions; i++ {
		// create GCM subscription with topic: gcmTopic
		response, errPost := http.Post(
			fmt.Sprintf(urlFormat, i, i, strings.TrimPrefix(fcmTopic, "/")),
			"text/plain",
			bytes.NewBufferString(""),
		)
		a.NoError(errPost)
		a.Equal(response.StatusCode, 200)

		body, errReadAll := ioutil.ReadAll(response.Body)
		a.NoError(errReadAll)
		a.Equal("subscribed: /topic\n", string(body))
	}
}

func (params *benchParams) tearDown() {
	assert.NoError(params, params.service.Stop())
	params.service = nil
	errRemove := os.RemoveAll(params.dir)
	if errRemove != nil {
		logger.WithError(errRemove).WithField("module", "testing").Error("Could not remove directory")
	}

}

func (params *benchParams) createClients() []client.Client {
	wsURL := "ws://" + params.service.WebServer().GetAddr() + "/stream/user/"

	clients := make([]client.Client, 0, params.clients)
	for clientID := 0; clientID < params.clients; clientID++ {
		location := wsURL + strconv.Itoa(clientID)
		c, err := client.Open(location, "http://localhost/", 1000, true)
		assert.NoError(params, err)
		clients = append(clients, c)
	}
	return clients
}

func (params *benchParams) throughputSend() {
	defer testutil.ResetDefaultRegistryHealthCheck()
	params.setUp()

	a := assert.New(params)
	clients := params.createClients()

	// Report allocations also
	params.ReportAllocs()
	logger.WithFields(log.Fields{
		"count": params.expectedMessagesCount(),
		"N":     params.N,
	}).Info("Expecting messages")
	params.wg.Add(params.expectedMessagesCount())

	// Reset timer to start the actual timing
	params.receiveLoop()
	params.ResetTimer()

	// wait until all messages are sent
	for _, c := range clients {
		go func(c client.Client) {
			for i := 0; i < params.N; i++ {
				a.NoError(params.send(c))
			}
		}(c)
	}
	params.wg.Wait()
	close(params.doneC)

	// stop timer after the actual test
	params.StopTimer()

	// stop service (and wait for all the messages to be processed during the given grace period)
	params.tearDown()
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

// messages per second
func (params *benchParams) mps() float64 {
	return float64(params.received) / params.duration().Seconds()
}
