package server

import (
	"bytes"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/server/fcm"
	"github.com/smancke/guble/server/service"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
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
	testTopic = "/topic"
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
	params.throughputFCM()
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
	params.throughputFCM()
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
	params.throughputFCM()
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
	params.throughputFCM()
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
	params.throughputFCM()
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
	params.throughputFCM()
	fmt.Println(params)
}

type sender func(c client.Client) error

func sendMessageSample(c client.Client) error {
	return c.Send(testTopic, "test-body", "{id:id}")
}

type benchParams struct {
	*testing.B
	workers       int           // number of fcm workers
	subscriptions int           // number of subscriptions listening on the topic
	timeout       time.Duration // fcm timeout response
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

func (params *benchParams) throughputFCM() {
	defer testutil.ResetDefaultRegistryHealthCheck()
	a := assert.New(params)

	dir, errTempDir := ioutil.TempDir("", "guble_benchmarking_fcm_test")
	a.NoError(errTempDir)

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
	if fcmConn == nil {
		a.FailNow("There should be a module of type: FCM Connector")
	}

	params.receiveC = make(chan bool)
	fcmConn.Sender = testutil.CreateFcmSender(
		testutil.CreateRoundTripperWithCountAndTimeout(http.StatusOK, testutil.SuccessFCMResponse, params.receiveC, params.timeout))

	urlFormat := fmt.Sprintf("http://%s/fcm/%%d/gcmId%%d/subscribe/%%s", params.service.WebServer().GetAddr())
	for i := 1; i <= params.subscriptions; i++ {
		// create FCM subscription
		response, errPost := http.Post(
			fmt.Sprintf(urlFormat, i, i, strings.TrimPrefix(testTopic, "/")),
			"text/plain",
			bytes.NewBufferString(""),
		)
		a.NoError(errPost)
		a.Equal(response.StatusCode, 200)

		body, errReadAll := ioutil.ReadAll(response.Body)
		a.NoError(errReadAll)
		a.Equal("{\"subscribed\":\"/topic\"}", string(body))
	}

	clients := params.createClients()

	// Report allocations also
	params.ReportAllocs()

	expectedMessagesNumber := params.N * params.clients * params.subscriptions
	logger.WithFields(log.Fields{
		"expectedMessagesNumber": expectedMessagesNumber,
		"N": params.N,
	}).Info("Expecting messages")
	params.wg.Add(expectedMessagesNumber)

	// start the receive loop (a select on receiveC and doneC)
	params.doneC = make(chan struct{})
	params.receiveLoop()

	params.ResetTimer()

	// send all messages, or fail on any error
	for _, cl := range clients {
		go func(cl client.Client) {
			for i := 0; i < params.N; i++ {
				err := params.sender(cl)
				if err != nil {
					a.FailNow("Message could not be sent")
				}
				params.sent++
			}
		}(cl)
	}

	// wait to receive all messages
	params.wg.Wait()

	// stop timer after the actual test
	params.StopTimer()

	close(params.doneC)
	a.NoError(params.service.Stop())
	params.service = nil
	close(params.receiveC)
	errRemove := os.RemoveAll(dir)
	if errRemove != nil {
		logger.WithError(errRemove).WithField("module", "testing").Error("Could not remove directory")
	}
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
