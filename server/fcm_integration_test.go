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

	"encoding/json"

	"github.com/cosminrentea/gobbler/client"
	"github.com/cosminrentea/gobbler/server/connector"
	"github.com/cosminrentea/gobbler/server/fcm"
	"github.com/cosminrentea/gobbler/server/service"
	"github.com/cosminrentea/gobbler/testutil"
	"github.com/stretchr/testify/assert"
)

var (
	testHttpPort         = 11000
	timeoutForOneMessage = 50 * time.Millisecond
)

type fcmMetricsMap struct {
	CurrentErrorsCount            int `json:"current_errors_count"`
	CurrentMessagesCount          int `json:"current_messages_count"`
	CurrentMessagesTotalLatencies int `json:"current_messages_total_latencies_nanos"`
	CurrentErrorsTotalLatencies   int `json:"current_errors_total_latencies_nanos"`
}

type fcmMetrics struct {
	TotalSentMessages      int           `json:"fcm.total_sent_messages"`
	TotalSentMessageErrors int           `json:"fcm.total_sent_message_errors"`
	Minute                 fcmMetricsMap `json:"fcm.minute"`
	Hour                   fcmMetricsMap `json:"fcm.hour"`
	Day                    fcmMetricsMap `json:"fcm.day"`
}

type routerMetrics struct {
	CurrentRoutes        int `json:"router.current_routes"`
	CurrentSubscriptions int `json:"router.current_subscriptions"`
}

type expectedValues struct {
	ZeroLatencies        bool
	MessageCount         int
	CurrentRoutes        int
	CurrentSubscriptions int
}

// Test that restarting the service continues to fetch messages from store for a subscription from lastID
func TestFCMRestart(t *testing.T) {
	defer testutil.EnableDebugForMethod()()
	defer testutil.ResetDefaultRegistryHealthCheck()

	a := assert.New(t)

	receiveC := make(chan bool)
	s, cleanup := serviceSetUp(t)
	defer cleanup()

	assertMetrics(a, s, expectedValues{true, 0, 0, 0})

	var fcmConn connector.ResponsiveConnector
	var ok bool
	for _, iface := range s.ModulesSortedByStartOrder() {
		fcmConn, ok = iface.(connector.ResponsiveConnector)
		if ok {
			break
		}
	}
	a.True(ok, "There should be a module of type FCMConnector")

	// add a high timeout so the messages are processed slow
	sender, err := fcm.CreateFcmSender(fcm.SuccessFCMResponse, receiveC, 10*time.Millisecond)
	a.NoError(err)
	fcmConn.SetSender(sender)

	// create subscription on topic
	subscriptionSetUp(t, s)

	assertMetrics(a, s, expectedValues{true, 0, 1, 1})

	c := clientSetUp(t, s)

	// send 3 messages in the router but read only one and close the service
	for i := 0; i < 3; i++ {
		c.Send(testTopic, "dummy body", "{dummy: value}")
	}

	// receive one message only from FCM
	select {
	case <-receiveC:
	case <-time.After(timeoutForOneMessage):
		a.Fail("Initial FCM message not received")
	}

	assertMetrics(a, s, expectedValues{false, 1, 1, 1})
	close(receiveC)
	// restart the service
	a.NoError(s.Stop())

	// remake the sender
	receiveC = make(chan bool)
	sender, err = fcm.CreateFcmSender(fcm.SuccessFCMResponse, receiveC, 10*time.Millisecond)
	a.NoError(err)
	fcmConn.SetSender(sender)

	time.Sleep(50 * time.Millisecond)
	testutil.ResetDefaultRegistryHealthCheck()
	a.NoError(s.Start())

	//TODO Cosmin Bogdan add 2 calls to assertMetrics before and after the next block

	// read the other 2 messages
	for i := 0; i < 1; i++ {
		select {
		case <-receiveC:
		case <-time.After(2 * timeoutForOneMessage):
			a.Fail("FCM message not received")
		}
	}
}

func serviceSetUp(t *testing.T) (*service.Service, func()) {
	dir, errTempDir := ioutil.TempDir("", "guble_fcm_test")
	assert.NoError(t, errTempDir)

	*Config.KVS = "memory"
	*Config.MS = "file"
	*Config.Cluster.NodeID = 0
	*Config.StoragePath = dir
	*Config.MetricsEndpoint = "/admin/metrics"
	*Config.WS.Enabled = true
	*Config.WS.Prefix = "/stream/"
	*Config.FCM.Enabled = true
	*Config.FCM.APIKey = "WILL BE OVERWRITTEN"
	*Config.FCM.Prefix = "/fcm/"
	*Config.FCM.Workers = 1 // use only one worker so we can control the number of messages that go to FCM
	*Config.APNS.Enabled = false

	var s *service.Service
	for s == nil {
		testHttpPort++
		logger.WithField("port", testHttpPort).Debug("trying to use HTTP Port")
		*Config.HttpListen = fmt.Sprintf("127.0.0.1:%d", testHttpPort)
		s = StartService()
	}
	return s, func() {
		errRemove := os.RemoveAll(dir)
		if errRemove != nil {
			logger.WithError(errRemove).WithField("module", "testing").Error("Could not remove directory")
		}
	}
}

func clientSetUp(t *testing.T, service *service.Service) client.Client {
	wsURL := "ws://" + service.WebServer().GetAddr() + "/stream/user/user01"
	c, err := client.Open(wsURL, "http://localhost/", 1000, false)
	assert.NoError(t, err)
	return c
}

func subscriptionSetUp(t *testing.T, service *service.Service) {
	a := assert.New(t)

	urlFormat := fmt.Sprintf("http://%s/fcm/%%d/gcmId%%d/%%s", service.WebServer().GetAddr())
	// create GCM subscription
	response, errPost := http.Post(
		fmt.Sprintf(urlFormat, 1, 1, strings.TrimPrefix(testTopic, "/")),
		"text/plain",
		bytes.NewBufferString(""),
	)
	a.NoError(errPost)
	a.Equal(response.StatusCode, 200)

	body, errReadAll := ioutil.ReadAll(response.Body)
	a.NoError(errReadAll)
	a.Equal(fmt.Sprintf(`{"subscribed":"%s"}`, testTopic), string(body))
}

func assertMetrics(a *assert.Assertions, s *service.Service, expected expectedValues) {
	httpClient := &http.Client{}
	u := fmt.Sprintf("http://%s%s", s.WebServer().GetAddr(), defaultMetricsEndpoint)
	request, err := http.NewRequest(http.MethodGet, u, nil)
	a.NoError(err)

	response, err := httpClient.Do(request)
	a.NoError(err)
	defer response.Body.Close()

	a.Equal(http.StatusOK, response.StatusCode)
	bodyBytes, err := ioutil.ReadAll(response.Body)
	a.NoError(err)
	logger.WithField("body", string(bodyBytes)).Debug("metrics response")

	mFCM := &fcmMetrics{}
	err = json.Unmarshal(bodyBytes, mFCM)
	a.NoError(err)

	a.Equal(0, mFCM.TotalSentMessageErrors)
	a.Equal(expected.MessageCount, mFCM.TotalSentMessages)

	a.Equal(0, mFCM.Minute.CurrentErrorsCount)
	a.Equal(expected.MessageCount, mFCM.Minute.CurrentMessagesCount)
	a.Equal(0, mFCM.Minute.CurrentErrorsTotalLatencies)
	a.Equal(expected.ZeroLatencies, mFCM.Minute.CurrentMessagesTotalLatencies == 0)

	a.Equal(0, mFCM.Hour.CurrentErrorsCount)
	a.Equal(expected.MessageCount, mFCM.Hour.CurrentMessagesCount)
	a.Equal(0, mFCM.Hour.CurrentErrorsTotalLatencies)
	a.Equal(expected.ZeroLatencies, mFCM.Hour.CurrentMessagesTotalLatencies == 0)

	a.Equal(0, mFCM.Day.CurrentErrorsCount)
	a.Equal(expected.MessageCount, mFCM.Day.CurrentMessagesCount)
	a.Equal(0, mFCM.Day.CurrentErrorsTotalLatencies)
	a.Equal(expected.ZeroLatencies, mFCM.Day.CurrentMessagesTotalLatencies == 0)

	mRouter := &routerMetrics{}
	err = json.Unmarshal(bodyBytes, mRouter)
	a.NoError(err)

	a.Equal(expected.CurrentRoutes, mRouter.CurrentRoutes)
	a.Equal(expected.CurrentSubscriptions, mRouter.CurrentSubscriptions)
}
