package gubled

import (
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server"

	"github.com/stretchr/testify/assert"
	"testing"

	"encoding/json"
	"github.com/smancke/guble/testutil"
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

func initServerAndClients(t *testing.T) (*server.Service, client.Client, client.Client, func()) {
	service := StartupService(Args{Listen: "localhost:0", KVBackend: "memory"})

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
