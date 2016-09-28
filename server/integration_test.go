package server

import (
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/service"

	"github.com/stretchr/testify/assert"

	"bytes"
	"encoding/json"
	"fmt"
	"github.com/smancke/guble/restclient"
	"github.com/smancke/guble/testutil"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"
)

// func TestSimplePingPong(t *testing.T) {
// 	defer testutil.ResetDefaultRegistryHealthCheck()
// 	testutil.ResetDefaultRegistryHealthCheck()

// 	_, client1, client2, tearDown := initServerAndClients(t)
// 	defer tearDown()

// 	client1.Subscribe("/foo")
// 	expectStatusMessage(t, client1, protocol.SUCCESS_SUBSCRIBED_TO, "/foo")

// 	client2.Send("/foo 42", "Hello", `{"key": "value"}`)
// 	expectStatusMessage(t, client2, protocol.SUCCESS_SEND, "42")

// 	select {
// 	case msg := <-client1.Messages():
// 		assert.Equal(t, "Hello", msg.BodyAsString())
// 		assert.Equal(t, "user2", msg.UserID)
// 		assert.Equal(t, `{"key": "value"}`, msg.HeaderJSON)
// 		assert.Equal(t, uint64(1), msg.ID)
// 	case msg := <-client1.Errors():
// 		t.Logf("received error: %v", msg)
// 		t.FailNow()
// 	case <-time.After(time.Millisecond * 100):
// 		t.Log("no message received")
// 		t.FailNow()
// 	}
// }

func initServerAndClients(t *testing.T) (*service.Service, client.Client, client.Client, func()) {
	*Config.HttpListen = "localhost:0"
	*Config.KVS = "memory"
	service := StartService()

	time.Sleep(time.Millisecond * 100)

	var err error
	client1, err := client.Open("ws://"+service.WebServer().GetAddr()+"/stream/user/user1", "http://localhost", 1, false)
	assert.NoError(t, err)

	checkConnectedNotificationJSON(t, "user1",
		expectStatusMessage(t, client1, protocol.SUCCESS_CONNECTED, "You are connected to the server."),
	)

	client2, err := client.Open("ws://"+service.WebServer().GetAddr()+"/stream/user/user2", "http://localhost", 1, false)
	assert.NoError(t, err)
	checkConnectedNotificationJSON(t, "user2",
		expectStatusMessage(t, client2, protocol.SUCCESS_CONNECTED, "You are connected to the server."),
	)

	return service, client1, client2, func() {
		if client1 != nil {
			client1.Close()
		}
		if client2 != nil {
			client2.Close()
		}
		service.Stop()
	}
}

func expectStatusMessage(t *testing.T, client client.Client, name string, arg string) string {
	select {
	case notify := <-client.StatusMessages():
		assert.Equal(t, name, notify.Name)
		assert.Equal(t, arg, notify.Arg)
		return notify.Json
	case <-time.After(time.Second * 10):
		t.Logf("no notification of type %s after 2 second", name)
		t.Fail()
		return ""
	}
}

func checkConnectedNotificationJSON(t *testing.T, user string, connectedJSON string) {
	m := make(map[string]string)
	err := json.Unmarshal([]byte(connectedJSON), &m)
	assert.NoError(t, err)
	assert.Equal(t, user, m["UserId"])
	assert.True(t, len(m["ApplicationId"]) > 0)
	_, e := time.Parse(time.RFC3339, m["Time"])
	assert.NoError(t, e)
}

//Used only for test and Unmarshall of the json response
type Subscriber struct {
	DeviceID string `json:"device_id"`
	UserID   string `json:"user_id"`
}

func Test_FranzIntegration(t *testing.T) {
	defer testutil.ResetDefaultRegistryHealthCheck()
	defer testutil.EnableDebugForMethod()()

	a := assert.New(t)

	s, cleanup := serviceSetUp(t)
	defer cleanup()

	subscribeMultipleClients(t, s, 4)
	a.Nil(nil)

	restClient := restclient.New(fmt.Sprintf("http://%s/api", s.WebServer().GetAddr()))
	content, err := restClient.GetSubscribers(testTopic)
	a.NoError(err)
	routeParams := make([]*Subscriber, 0)

	err = json.Unmarshal(content, &routeParams)
	a.Equal(4, len(routeParams), "Should have 4 subscribers")
	for i, rp := range routeParams {
		a.Equal(fmt.Sprintf("gcmId%d", i), rp.DeviceID)
		a.Equal(fmt.Sprintf("%d", i), rp.UserID)
	}
	a.NoError(err)
}

func subscribeMultipleClients(t *testing.T, service *service.Service, noOfClients int) {
	a := assert.New(t)

	// create FCM subscription for topic
	for i := 0; i < noOfClients; i++ {
		urlFormat := fmt.Sprintf("http://%s/gcm/%%d/gcmId%%d/subscribe/%%s", service.WebServer().GetAddr())
		url := fmt.Sprintf(urlFormat, i, i, strings.TrimPrefix(testTopic, "/"))
		response, errPost := http.Post(
			url,
			"text/plain",
			bytes.NewBufferString(""),
		)
		logger.WithField("url", url).Debug("subscribe")
		a.NoError(errPost)
		a.Equal(response.StatusCode, 200)

		body, errReadAll := ioutil.ReadAll(response.Body)
		a.NoError(errReadAll)
		a.Equal(fmt.Sprintf(`{"subscribed":"%s"}`, testTopic), string(body))
	}
}
