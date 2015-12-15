package main

import (
	assert "github.com/stretchr/testify/assert"
	"testing"

	"github.com/smancke/guble/client"
	"github.com/smancke/guble/server"
	"time"
)

var ws *server.WSServer
var client1 *client.Client
var client2 *client.Client

func TestSimplePingPong(t *testing.T) {
	defer initServerAndClients(t)()

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

func initServerAndClients(t *testing.T) func() {
	router := server.NewPubSubRouter().Go()
	messageEntry := server.NewMessageEntry(router)
	wshandlerFactory := func(wsConn server.WSConn, userId string) server.Startable {
		return server.NewWSHandler(router, messageEntry, wsConn, userId)
	}
	ws = server.StartWSServer("localhost:0", "/", wshandlerFactory)

	time.Sleep(time.Millisecond * 100)

	var err error
	client1, err = client.Open("ws://"+ws.GetAddr()+"/user/user1", "http://localhost", 1, false)
	assert.NoError(t, err)
	client2, err = client.Open("ws://"+ws.GetAddr()+"/user/user2", "http://localhost", 1, false)
	assert.NoError(t, err)

	return func() {
		router.Stop()
		ws.Stop()
		if client1 != nil {
			client1.Close()
		}
		if client2 != nil {
			client2.Close()
		}
	}
}
