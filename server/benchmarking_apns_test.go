package server

import (
	"bytes"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/server/connector"
	"github.com/smancke/guble/testutil"
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
	defer testutil.ResetDefaultRegistryHealthCheck()
	a := assert.New(params)

	dir, errTempDir := ioutil.TempDir("", "guble_benchmarking_apns_test")
	a.NoError(errTempDir)

	*Config.HttpListen = "localhost:0"
	*Config.KVS = "memory"
	*Config.MS = "file"
	*Config.StoragePath = dir
	*Config.APNS.Enabled = true

	params.service = StartService()

	var apnsConn connector.ReactiveConnector
	var ok bool
	for _, iface := range params.service.ModulesSortedByStartOrder() {
		apnsConn, ok = iface.(connector.ReactiveConnector)
		if ok {
			break
		}
	}
	if apnsConn == nil {
		a.FailNow("There should be a module of type: APNS Connector")
	}

	params.receiveC = make(chan bool)

	//TODO Cosmin replace with: setting the sender
	apnsConn.Check()
	//apnsConn.Sender = testutil.CreateFcmSender(
	//	testutil.CreateRoundTripperWithCountAndTimeout(http.StatusOK, testutil.SuccessFCMResponse, params.receiveC, params.timeout))

	urlFormat := fmt.Sprintf("http://%s/apns/%%d/gcmId%%d/subscribe/%%s", params.service.WebServer().GetAddr())
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
