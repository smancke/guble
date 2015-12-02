package server

import (
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/websocket"
	"testing"
	"time"
)

func TestStartAndStopWSServer(t *testing.T) {
	defer initCtrl(t)()

	startable := NewMockStartable(ctrl)
	startable.EXPECT().Start().Do(func() {
		time.Sleep(time.Second * 10)
	}).AnyTimes()
	var conn WSConn
	newWsHandler := func(wsConn WSConn) Startable {
		conn = wsConn
		return startable
	}
	server := StartWSServer("localhost:0", newWsHandler)
	time.Sleep(time.Millisecond * 10)

	addr := server.GetAddr()

	sendTestMessage(t, addr, &conn)

	server.Stop()

	time.Sleep(time.Millisecond * 10)

	_, err := websocket.Dial("ws://"+addr, "", "http://localhost/")
	assert.Error(t, err) // server closed
}

func sendTestMessage(t *testing.T, addr string, conn *WSConn) {
	ws, err := websocket.Dial("ws://"+addr, "", "http://localhost/")
	assert.NoError(t, err)
	defer ws.Close()

	_, err = ws.Write([]byte("Testmessage"))
	assert.NoError(t, err)

	var msg []byte
	connDeref := *conn
	err = connDeref.Receive(&msg)
	assert.NoError(t, err)
	assert.Equal(t, "Testmessage", string(msg))
}
