package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/cosminrentea/gobbler/client"
	"github.com/cosminrentea/gobbler/server/connector"
	"github.com/cosminrentea/gobbler/server/fcm"
	"github.com/cosminrentea/gobbler/testutil"
	"github.com/stretchr/testify/assert"
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

	var fcmConn connector.ResponsiveConnector
	var ok bool
	for _, iface := range params.service.ModulesSortedByStartOrder() {
		fcmConn, ok = iface.(connector.ResponsiveConnector)
		if ok {
			break
		}
	}
	if fcmConn == nil {
		a.FailNow("There should be a module of type: FCM Connector")
	}

	params.receiveC = make(chan bool)
	sender, err := fcm.CreateFcmSender(fcm.SuccessFCMResponse, params.receiveC, params.timeout)
	a.NoError(err)
	fcmConn.SetSender(sender)

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
