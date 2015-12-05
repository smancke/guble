package main

import (
	assert "github.com/stretchr/testify/assert"
	"testing"

	"github.com/smancke/guble/client"
	"github.com/smancke/guble/server"
	"time"
)

var testListen = "localhost:9999"
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
	case msg := <-client1.Errors():
		t.Logf("received error: %v", msg)
		t.FailNow()
	case <-time.After(time.Millisecond * 100):
		t.Log("no message received")
		t.FailNow()
	}
}

func initServerAndClients(t *testing.T) func() {
	mux := server.NewPubSubRouter().Go()
	wshandlerFactory := func(wsConn server.WSConn) server.Startable {
		return server.NewWSHandler(mux, mux, wsConn)
	}
	ws = server.StartWSServer(testListen, wshandlerFactory)

	time.Sleep(time.Millisecond * 10)

	var err error
	client1, err = client.Open("ws://"+testListen, "http://localhost/", 1, false)
	assert.NoError(t, err)
	client2, err = client.Open("ws://"+testListen, "http://localhost/", 1, false)
	assert.NoError(t, err)

	return func() {
		mux.Stop()
		ws.Stop()
		if client1 != nil {
			client1.Close()
		}
		if client2 != nil {
			client2.Close()
		}
	}
}
