package gubled

import (
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/server"

	assert "github.com/stretchr/testify/assert"
	"testing"

	"time"
)

func TestSimplePingPong(t *testing.T) {
	_, client1, client2, tearDown := initServerAndClients(t)
	defer tearDown()

	client1.Subscribe("/foo")
	time.Sleep(time.Millisecond * 10)
	client2.Send("/foo", "Hallo")

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
	client1, err := client.Open("ws://"+service.GetWebServer().GetAddr()+"/user/user1", "http://localhost", 1, false)
	assert.NoError(t, err)
	client2, err := client.Open("ws://"+service.GetWebServer().GetAddr()+"/user/user2", "http://localhost", 1, false)
	assert.NoError(t, err)

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
