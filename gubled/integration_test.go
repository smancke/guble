package gubled

import (
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/server"

	assert "github.com/stretchr/testify/assert"
	"testing"

	"encoding/json"
	"time"
)

func TestSimplePingPong(t *testing.T) {
	_, client1, client2, tearDown := initServerAndClients(t)
	defer tearDown()

	client1.Subscribe("/foo")
	expectStatusMessage(t, client1, guble.SUCCESS_SUBSCRIBED_TO, "/foo")

	time.Sleep(time.Millisecond * 10)
	client2.Send("/foo 42", "Hallo")
	expectStatusMessage(t, client2, guble.SUCCESS_SEND, "42")

	select {
	case msg := <-client1.Messages():
		assert.Equal(t, "Hallo", msg.BodyAsString())
		assert.Equal(t, "user2", msg.PublisherUserId)
		assert.Equal(t, uint64(1), msg.Id)
	case msg := <-client1.Errors():
		t.Logf("received error: %v", msg)
		t.FailNow()
	case <-time.After(time.Millisecond * 100):
		t.Log("no message received")
		t.FailNow()
	}
}

func initServerAndClients(t *testing.T) (*server.Service, *client.Client, *client.Client, func()) {
	service := StartupService(Args{Listen: "localhost:0"})

	time.Sleep(time.Millisecond * 100)

	var err error
	client1, err := client.Open("ws://"+service.GetWebServer().GetAddr()+"/stream/user/user1", "http://localhost", 1, false)
	assert.NoError(t, err)

	checkConnectedNotificationJson(t, "user1",
		expectStatusMessage(t, client1, guble.SUCCESS_CONNECTED, "You are connected to the server."),
	)

	client2, err := client.Open("ws://"+service.GetWebServer().GetAddr()+"/stream/user/user2", "http://localhost", 1, false)
	assert.NoError(t, err)
	checkConnectedNotificationJson(t, "user2",
		expectStatusMessage(t, client2, guble.SUCCESS_CONNECTED, "You are connected to the server."),
	)

	return service, client1, client2, func() {
		service.Stop()

		if client1 != nil {
			client1.Close()
		}
		if client2 != nil {
			client2.Close()
		}
	}
}

func expectStatusMessage(t *testing.T, client *client.Client, name string, arg string) string {
	select {
	case notify := <-client.StatusMessages():
		assert.Equal(t, name, notify.Name)
		assert.Equal(t, arg, notify.Arg)
		return notify.Json
	case <-time.After(time.Second * 1):
		t.Logf("no notification of type %s after 1 second", name)
		t.Fail()
		return ""
	}
}

func checkConnectedNotificationJson(t *testing.T, user string, connectedJson string) {
	m := make(map[string]string)
	err := json.Unmarshal([]byte(connectedJson), &m)
	assert.NoError(t, err)
	assert.Equal(t, user, m["UserId"])
	assert.True(t, len(m["ApplicationId"]) > 0)
	_, e := time.Parse(time.RFC3339, m["Time"])
	assert.NoError(t, e)
}
