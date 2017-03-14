package server

import (
	"bytes"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/golang/mock/gomock"
	"github.com/sideshow/apns2"
	"github.com/cosminrentea/gobbler/client"
	"github.com/cosminrentea/gobbler/server/apns"
	"github.com/cosminrentea/gobbler/server/connector"
	"github.com/cosminrentea/gobbler/server/router"
	"github.com/cosminrentea/gobbler/server/websocket"
	"github.com/cosminrentea/gobbler/testutil"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// APNS benchmarks
func BenchmarkAPNS_1Workers50MilliTimeout(b *testing.B) {
	params := &benchParams{
		B:             b,
		workers:       1,
		subscriptions: 8,
		timeout:       50 * time.Millisecond,
		clients:       8,
		sender:        sendMessageSample,
	}
	params.throughputAPNS()
	fmt.Println(params)
}

func BenchmarkAPNS_8Workers50MilliTimeout(b *testing.B) {
	params := &benchParams{
		B:             b,
		workers:       8,
		subscriptions: 8,
		timeout:       50 * time.Millisecond,
		clients:       8,
		sender:        sendMessageSample,
	}
	params.throughputAPNS()
	fmt.Println(params)
}

func BenchmarkAPNS_16Workers50MilliTimeout(b *testing.B) {
	params := &benchParams{
		B:             b,
		workers:       16,
		subscriptions: 8,
		timeout:       50 * time.Millisecond,
		clients:       8,
		sender:        sendMessageSample,
	}
	params.throughputAPNS()
	fmt.Println(params)
}

func BenchmarkAPNS_1Workers100MilliTimeout(b *testing.B) {
	params := &benchParams{
		B:             b,
		workers:       1,
		subscriptions: 8,
		timeout:       100 * time.Millisecond,
		clients:       8,
		sender:        sendMessageSample,
	}
	params.throughputAPNS()
	fmt.Println(params)
}

func BenchmarkAPNS_8Workers100MilliTimeout(b *testing.B) {
	params := &benchParams{
		B:             b,
		workers:       8,
		subscriptions: 8,
		timeout:       100 * time.Millisecond,
		clients:       8,
		sender:        sendMessageSample,
	}
	params.throughputAPNS()
	fmt.Println(params)
}

func BenchmarkAPNS_16Workers100MilliTimeout(b *testing.B) {
	params := &benchParams{
		B:             b,
		workers:       16,
		subscriptions: 8,
		timeout:       100 * time.Millisecond,
		clients:       8,
		sender:        sendMessageSample,
	}
	params.throughputAPNS()
	fmt.Println(params)
}

func (params *benchParams) throughputAPNS() {
	defer testutil.EnableDebugForMethod()()
	_, finish := testutil.NewMockBenchmarkCtrl(params.B)
	defer finish()
	defer testutil.ResetDefaultRegistryHealthCheck()
	a := assert.New(params)

	dir, errTempDir := ioutil.TempDir("", "guble_benchmarking_apns_test")
	a.NoError(errTempDir)

	*Config.HttpListen = "localhost:0"
	*Config.KVS = "memory"
	*Config.MS = "file"
	*Config.StoragePath = dir
	*Config.APNS.Enabled = true
	*Config.APNS.AppTopic = "app.topic"
	*Config.APNS.Prefix = "/apns/"

	params.receiveC = make(chan bool)
	CreateModules = createModulesWebsocketAndMockAPNSPusher(params.receiveC, params.timeout)

	params.service = StartService()

	var apnsConn connector.ResponsiveConnector
	var ok bool
	for _, iface := range params.service.ModulesSortedByStartOrder() {
		apnsConn, ok = iface.(connector.ResponsiveConnector)
		if ok {
			break
		}
	}
	if apnsConn == nil {
		a.FailNow("There should be a module of type: APNS Connector")
	}

	urlFormat := fmt.Sprintf("http://%s/apns/apns-%%d/%%d/%%s", params.service.WebServer().GetAddr())
	for i := 1; i <= params.subscriptions; i++ {
		// create APNS subscription
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
		"b.N": params.N,
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

var createModulesWebsocketAndMockAPNSPusher = func(receiveC chan bool, simulatedLatency time.Duration) func(router router.Router) []interface{} {
	return func(router router.Router) []interface{} {
		var modules []interface{}

		if wsHandler, err := websocket.NewWSHandler(router, "/stream/"); err != nil {
			logger.WithError(err).Error("Error loading WSHandler module")
		} else {
			modules = append(modules, wsHandler)
		}

		if *Config.APNS.Enabled {
			if *Config.APNS.AppTopic == "" {
				logger.Panic("The Mobile App Topic (usually the bundle-id) has to be provided when APNS is enabled")
			}

			// create and use a mock Pusher - introducing a latency per each message
			rsp := &apns2.Response{
				ApnsID:     "apns-id",
				StatusCode: 200,
			}
			mPusher := NewMockPusher(testutil.MockCtrl)
			mPusher.EXPECT().Push(gomock.Any()).
				Do(func(notif *apns2.Notification) (*apns2.Response, error) {
					time.Sleep(simulatedLatency)
					receiveC <- true
					return nil, nil
				}).Return(rsp, nil).AnyTimes()

			apnsSender, err := apns.NewSenderUsingPusher(mPusher, *Config.APNS.AppTopic)
			if err != nil {
				logger.Panic("APNS Sender could not be created")
			}
			if apnsConn, err := apns.New(router, apnsSender, Config.APNS); err != nil {
				logger.WithError(err).Error("Error creating APNS connector")
			} else {
				modules = append(modules, apnsConn)
			}
		} else {
			logger.Info("APNS: disabled")
		}

		return modules
	}
}
