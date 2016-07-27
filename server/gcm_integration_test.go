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

	"github.com/smancke/guble/client"
	"github.com/smancke/guble/server/gcm"
	"github.com/smancke/guble/server/service"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
)

var (
	testTopic = "/path"
)

// Test that restarting the service continues to fetch messages from store
// for a subscription from lastID
func TestGCM_Restart(t *testing.T) {
	//	defer testutil.EnableDebugForMethod()()
	defer testutil.ResetDefaultRegistryHealthCheck()

	a := assert.New(t)

	receiveC := make(chan bool)
	service := serviceSetUp(t)

	var gcmConnector *gcm.Connector
	var ok bool
	for _, iface := range service.ModulesSortedByStartOrder() {
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
	subscriptionSetUp(t, service)

	client := clientSetUp(t, service)

	// send 3 messages in the router but read only one and close the service
	for i := 0; i < 3; i++ {
		client.Send(testTopic, "dummy body", "{dummy: value}")
	}

	// receive one message only from GCM
	select {
	case <-receiveC:
		return
	case <-time.After(50 * time.Millisecond):
		a.Fail("GCM message not received")
	}

	// restart the service
	a.NoError(service.Stop())

	time.Sleep(100 * time.Millisecond)

	a.NoError(service.Start())

	newReceiveC := make(chan bool)

	for _, iface := range service.ModulesSortedByStartOrder() {
		gcmConnector, ok = iface.(*gcm.Connector)
		if ok {
			break
		}
	}
	a.True(ok, "There should be a module of type GCMConnector")

	// add a high timeout so the messages are processed slow
	gcmConnector.Sender = testutil.CreateGcmSender(
		testutil.CreateRoundTripperWithCountAndTimeout(
			http.StatusOK, testutil.SuccessGCMResponse, newReceiveC, 10*time.Millisecond))

	// read the other 2 messages
	for i := 0; i < 2; i++ {
		select {
		case <-newReceiveC:
			return
		case <-time.After(50 * time.Millisecond):
			a.Fail("GCM message not received")
		}
	}
}

func serviceSetUp(t *testing.T) *service.Service {
	dir, errTempDir := ioutil.TempDir("", "guble_gcm_test")
	defer func() {
		errRemove := os.RemoveAll(dir)
		if errRemove != nil {
			logger.WithError(errRemove).WithField("module", "testing").Error("Could not remove directory")
		}
	}()
	assert.NoError(t, errTempDir)

	*config.HttpListen = "localhost:0"
	*config.KVS = "memory"
	*config.MS = "file"
	*config.Cluster.NodeID = 0
	*config.StoragePath = dir
	*config.GCM.Enabled = true
	*config.GCM.APIKey = "WILL BE OVERWRITTEN"
	*config.GCM.Workers = 1 // use only one worker so we can control the number of messages that go to GCM

	service := StartService()
	return service
}

func clientSetUp(t *testing.T, service *service.Service) client.Client {
	wsURL := "ws://" + service.WebServer().GetAddr() + "/stream/user/user01"
	client, err := client.Open(wsURL, "http://localhost/", 1000, false)
	assert.NoError(t, err)
	return client
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
	a.Equal(fmt.Sprintf("registered: %s\n", testTopic), string(body))

}
