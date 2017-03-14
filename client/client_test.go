package client

import (
	"github.com/cosminrentea/gobbler/testutil"

	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

var aNormalMessage = `/foo/bar,42,user01,phone01,{},1420110000,0

Hello World`

var aSendNotification = "#send"

var anErrorNotification = "!error-send"

func MockConnectionFactory(connectionMock *MockWSConnection) func(string, string) (WSConnection, error) {
	return func(url string, origin string) (WSConnection, error) {
		return connectionMock, nil
	}
}

func TestConnectErrorWithoutReconnection(t *testing.T) {
	a := assert.New(t)

	// given a client
	c := New("url", "origin", 1, false)

	// which raises an error on connect
	callCounter := 0
	c.SetWSConnectionFactory(func(url string, origin string) (WSConnection, error) {
		a.Equal("url", url)
		a.Equal("origin", origin)
		callCounter++
		return nil, fmt.Errorf("emulate connection error")
	})

	// when we start
	err := c.Start()

	// then
	a.Error(err)
	a.Equal(1, callCounter)
}

func TestConnectErrorWithoutReconnectionUsingOpen(t *testing.T) {
	a := assert.New(t)

	c, err := Open("url", "origin", 1, false)

	// which raises an error on connect
	callCounter := 0
	c.SetWSConnectionFactory(func(url string, origin string) (WSConnection, error) {
		a.Equal("url", url)
		a.Equal("origin", origin)
		callCounter++
		return nil, fmt.Errorf("emulate connection error")
	})

	a.Error(err)
}

func TestConnectErrorWithReconnection(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	// given a client
	c := New("url", "origin", 1, true)

	// which raises an error twice and then allows to connect
	callCounter := 0
	connMock := NewMockWSConnection(ctrl)
	connMock.EXPECT().ReadMessage().Do(func() { time.Sleep(time.Second) })
	c.SetWSConnectionFactory(func(url string, origin string) (WSConnection, error) {
		a.Equal("url", url)
		a.Equal("origin", origin)
		if callCounter <= 2 {
			callCounter++
			return nil, fmt.Errorf("emulate connection error")
		}
		return connMock, nil
	})

	// when we start
	err := c.Start()

	// then we get an error, first
	a.Error(err)
	a.False(c.IsConnected())

	// when we wait for two iterations and 10ms buffer time to connect
	time.Sleep(time.Millisecond * 110)

	// then we got connected
	a.True(c.IsConnected())
	a.Equal(3, callCounter)
}

func TestStopableClient(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	// given a client
	c := New("url", "origin", 1, true)

	// with a closeable connection
	connMock := NewMockWSConnection(ctrl)
	close := make(chan bool, 1)
	connMock.EXPECT().ReadMessage().
		Do(func() { <-close }).
		Return(0, []byte{}, fmt.Errorf("expected close error"))

	connMock.EXPECT().Close().Do(func() {
		close <- true
	})

	c.SetWSConnectionFactory(MockConnectionFactory(connMock))

	// when we start
	err := c.Start()

	// than we are connected
	a.NoError(err)
	a.True(c.IsConnected())

	// when we clode
	c.Close()
	time.Sleep(time.Millisecond * 1)

	// than the client returns
	a.False(c.IsConnected())
}

func TestReceiveAMessage(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	// given a client
	c := New("url", "origin", 10, false)

	// with a closeable connection
	connMock := NewMockWSConnection(ctrl)
	close := make(chan bool, 1)

	// normal message
	call1 := connMock.EXPECT().ReadMessage().
		Return(4, []byte(aNormalMessage), nil)
	call2 := connMock.EXPECT().ReadMessage().
		Return(4, []byte(aSendNotification), nil)
	call3 := connMock.EXPECT().ReadMessage().
		Return(4, []byte("---"), nil)
	call4 := connMock.EXPECT().ReadMessage().
		Return(4, []byte(anErrorNotification), nil)
	call5 := connMock.EXPECT().ReadMessage().
		Do(func() { <-close }).
		Return(0, []byte{}, fmt.Errorf("expected close error")).
		AnyTimes()

	call5.After(call4)
	call4.After(call3)
	call3.After(call2)
	call2.After(call1)

	c.SetWSConnectionFactory(MockConnectionFactory(connMock))

	connMock.EXPECT().Close().Do(func() {
		close <- true
	})

	// when we start
	err := c.Start()
	a.NoError(err)
	a.True(c.IsConnected())

	// than we receive the expected message
	select {
	case m := <-c.Messages():
		a.Equal(aNormalMessage, string(m.Bytes()))
	case <-time.After(time.Millisecond * 10):
		a.Fail("timeout while waiting for message")
	}

	// and we receive the notification
	select {
	case m := <-c.StatusMessages():
		a.Equal(aSendNotification, string(m.Bytes()))
	case <-time.After(time.Millisecond * 10):
		a.Fail("timeout while waiting for message")
	}

	// parse error
	select {
	case m := <-c.Errors():
		a.True(strings.HasPrefix(string(m.Bytes()), "!clientError "))
	case <-time.After(time.Millisecond * 10):
		a.Fail("timeout while waiting for message")
	}

	// and we receive the error notification
	select {
	case m := <-c.Errors():
		a.Equal(anErrorNotification, string(m.Bytes()))
	case <-time.After(time.Millisecond * 10):
		a.Fail("timeout while waiting for message")
	}

	c.Close()
}

func TestSendAMessage(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	//	a := assert.New(t)

	// given a client
	c := New("url", "origin", 1, true)

	// when expects a message
	connMock := NewMockWSConnection(ctrl)
	connMock.EXPECT().WriteMessage(websocket.BinaryMessage, []byte("> /foo\n{}\nTest"))
	connMock.EXPECT().
		ReadMessage().
		Return(websocket.BinaryMessage, []byte(aNormalMessage), nil).
		Do(func() {
			time.Sleep(time.Millisecond * 50)
		}).
		AnyTimes()
	c.SetWSConnectionFactory(MockConnectionFactory(connMock))

	c.Start()
	// then the expectation is meet by sending it
	c.Send("/foo", "Test", "{}")
	// stop client after 200ms
	time.AfterFunc(time.Millisecond*200, func() { c.Close() })
}

func TestSendSubscribeMessage(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	// given a client
	c := New("url", "origin", 1, true)

	// when expects a message
	connMock := NewMockWSConnection(ctrl)
	connMock.EXPECT().WriteMessage(websocket.BinaryMessage, []byte("+ /foo"))
	connMock.EXPECT().
		ReadMessage().
		Return(websocket.BinaryMessage, []byte(aNormalMessage), nil).
		Do(func() {
			time.Sleep(time.Millisecond * 50)
		}).
		AnyTimes()
	c.SetWSConnectionFactory(MockConnectionFactory(connMock))

	c.Start()
	c.Subscribe("/foo")

	// stop client after 200ms
	time.AfterFunc(time.Millisecond*200, func() { c.Close() })
}

func TestSendUnSubscribeMessage(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	// given a client
	c := New("url", "origin", 1, true)

	// when expects a message
	connMock := NewMockWSConnection(ctrl)
	connMock.EXPECT().WriteMessage(websocket.BinaryMessage, []byte("- /foo"))
	connMock.EXPECT().
		ReadMessage().
		Return(websocket.BinaryMessage, []byte(aNormalMessage), nil).
		Do(func() {
			time.Sleep(time.Millisecond * 50)
		}).
		AnyTimes()
	c.SetWSConnectionFactory(MockConnectionFactory(connMock))

	c.Start()
	c.Unsubscribe("/foo")

	// stop client after 200ms
	time.AfterFunc(time.Millisecond*200, func() { c.Close() })
}
