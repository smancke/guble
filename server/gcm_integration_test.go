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

	"github.com/smancke/guble/client"
	"github.com/smancke/guble/server/gcm"
	"github.com/smancke/guble/server/service"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
)

var (
	testTopic    = "/path"
	testHttpPort = 11000
)

type fcmMetricsMap struct {
	CurrentErrorsCount            int `json:"current_errors_count"`
	CurrentMessagesCount          int `json:"current_messages_count"`
	CurrentMessagesTotalLatencies int `json:"current_messages_total_latencies_nanos"`
	CurrentErrorsTotalLatencies   int `json:"current_errors_total_latencies_nanos"`
}

type fcmMetrics struct {
	TotalSentMessages      int           `json:"guble.fcm.total_sent_messages"`
	TotalSentMessageErrors int           `json:"guble.fcm.total_sent_message_errors"`
	Minute                 fcmMetricsMap `json:"guble.fcm.minute"`
	Hour                   fcmMetricsMap `json:"guble.fcm.hour"`
	Day                    fcmMetricsMap `json:"guble.fcm.day"`
}

type routerMetrics struct {
	CurrentRoutes        int `json:"guble.router.current_routes"`
	CurrentSubscriptions int `json:"guble.router.current_subscriptions"`
}

type expectedValues struct {
	MessageCount         int
	CurrentRoutes        int
	CurrentSubscriptions int
}

// Test that restarting the service continues to fetch messages from store
// for a subscription from lastID
func TestGCM_Restart(t *testing.T) {
	// defer testutil.EnableDebugForMethod()()
	defer testutil.ResetDefaultRegistryHealthCheck()

	a := assert.New(t)

	receiveC := make(chan bool)
	s, cleanup := serviceSetUp(t)
	defer cleanup()

	var gcmConnector *gcm.Connector
	var ok bool
	for _, iface := range s.ModulesSortedByStartOrder() {
		gcmConnector, ok = iface.(*gcm.Connector)
		if ok {
			break
		}
	}
	a.True(ok, "There should be a module of type GCMConnector")

	// add a high timeout so the messages are processed slow
	gcmConnector.Sender = testutil.CreateGcmSender(
		testutil.CreateRoundTripperWithCountAndTimeout(
			http.StatusOK, testutil.SuccessGCMResponse, receiveC, 10*time.Millisecond))

	// create subscription on topic
	subscriptionSetUp(t, s)

	c := clientSetUp(t, s)

	// send 3 messages in the router but read only one and close the service
	for i := 0; i < 3; i++ {
		c.Send(testTopic, "dummy body", "{dummy: value}")
	}

	// receive one message only from GCM
	select {
	case <-receiveC:
	case <-time.After(50 * time.Millisecond):
		a.Fail("Initial GCM message not received")
	}

	httpClient := &http.Client{}
	u := fmt.Sprintf("http://%s/admin/metrics", s.WebServer().GetAddr())
	request, err := http.NewRequest(http.MethodGet, u, nil)
	a.NoError(err)
	response, err := httpClient.Do(request)
	a.NoError(err)
	defer response.Body.Close()

	a.Equal(http.StatusOK, response.StatusCode)
	bodyBytes, err := ioutil.ReadAll(response.Body)
	a.NoError(err)
	logger.WithField("body", string(bodyBytes)).Debug("metrics response")

	assertMetrics(a, bodyBytes, expectedValues{1, 1, 1})

	// restart the service
	a.NoError(s.Stop())

	a.NoError(s.Start())

	// read the other 2 messages
	for i := 0; i < 2; i++ {
		select {
		case <-receiveC:
		case <-time.After(150 * time.Millisecond):
			a.Fail("GCM message not received")
		}
	}

	//TODO Cosmin after test code actually reaches here:
	// invoke metrics endpoint and assertMetrics once again
}

func serviceSetUp(t *testing.T) (*service.Service, func()) {
	dir, errTempDir := ioutil.TempDir("", "guble_gcm_test")
	assert.NoError(t, errTempDir)

	*config.KVS = "memory"
	*config.MS = "file"
	*config.Cluster.NodeID = 0
	*config.StoragePath = dir
	*config.MetricsEndpoint = "/admin/metrics"
	*config.GCM.Enabled = true
	*config.GCM.APIKey = "WILL BE OVERWRITTEN"
	*config.GCM.Workers = 1 // use only one worker so we can control the number of messages that go to GCM

	var s *service.Service
	for s == nil {
		testHttpPort++
		logger.WithField("port", testHttpPort).Debug("trying to use HTTP Port")
		*config.HttpListen = fmt.Sprintf("127.0.0.1:%d", testHttpPort)
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

	urlFormat := fmt.Sprintf("http://%s/gcm/%%d/gcmId%%d/subscribe/%%s", service.WebServer().GetAddr())
	// create GCM subscription with topic: gcmTopic
	response, errPost := http.Post(
		fmt.Sprintf(urlFormat, 1, 1, strings.TrimPrefix(testTopic, "/")),
		"text/plain",
		bytes.NewBufferString(""),
	)
	a.NoError(errPost)
	a.Equal(response.StatusCode, 200)

	body, errReadAll := ioutil.ReadAll(response.Body)
	a.NoError(errReadAll)
	a.Equal(fmt.Sprintf("subscribed: %s\n", testTopic), string(body))
}

func assertMetrics(a *assert.Assertions, bodyBytes []byte, expected expectedValues) {
	mFCM := &fcmMetrics{}
	err := json.Unmarshal(bodyBytes, mFCM)
	a.NoError(err)

	a.Equal(0, mFCM.TotalSentMessageErrors)
	a.Equal(expected.MessageCount, mFCM.TotalSentMessages)

	a.Equal(0, mFCM.Minute.CurrentErrorsCount)
	a.Equal(expected.MessageCount, mFCM.Minute.CurrentMessagesCount)
	a.Equal(0, mFCM.Minute.CurrentErrorsTotalLatencies)
	a.True(mFCM.Minute.CurrentMessagesTotalLatencies > 0)

	a.Equal(0, mFCM.Hour.CurrentErrorsCount)
	a.Equal(expected.MessageCount, mFCM.Hour.CurrentMessagesCount)
	a.Equal(0, mFCM.Hour.CurrentErrorsTotalLatencies)
	a.True(mFCM.Hour.CurrentMessagesTotalLatencies > 0)

	a.Equal(0, mFCM.Day.CurrentErrorsCount)
	a.Equal(expected.MessageCount, mFCM.Day.CurrentMessagesCount)
	a.Equal(0, mFCM.Day.CurrentErrorsTotalLatencies)
	a.True(mFCM.Day.CurrentMessagesTotalLatencies > 0)

	mRouter := &routerMetrics{}
	err = json.Unmarshal(bodyBytes, mRouter)
	a.NoError(err)

	a.Equal(expected.CurrentRoutes, mRouter.CurrentRoutes)
	a.Equal(expected.CurrentSubscriptions, mRouter.CurrentSubscriptions)
}
