package server

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
)

var pubSubSourceMock *PubSubSourceMock
var messageSinkMock *MessageSinkMock

func TestSubscriptionMessage(t *testing.T) {
	a := assert.New(t)

	// Given
	pubSubSourceMock = new(PubSubSourceMock)
	messageSinkMock = new(MessageSinkMock)
	handler := NewWSHandler(pubSubSourceMock, messageSinkMock)

	//handler := aMockedWSHandler()
	subscriptionPath := "/mock"
	wsconn := &WSConnMock{}
	wsconn.On("LocationString").
		Return(subscriptionPath)
	wsconn.On("Receive", mock.AnythingOfType("*[]uint8")).
		Return(nil)
	pubSubSourceMock.On("Subscribe", mock.AnythingOfType("*server.Route")).
		Return(nil)

	// when I
	//sendASubscribeMessage(handler, subscriptionPath)
	go func() {

		handler.HandleNewConnection(wsconn)
	}()

	time.Sleep(time.Millisecond * 10)

	// then the mock was called for subscription
	// TODO
	a.Equal([]*mock.Call{}, pubSubSourceMock.ExpectedCalls)
}

func aMockedWSHandler() *WSHandler {
	pubSubSourceMock = new(PubSubSourceMock)
	messageSinkMock = new(MessageSinkMock)
	return NewWSHandler(pubSubSourceMock, messageSinkMock)
}

func sendASubscribeMessage(handler *WSHandler, path string) {

}

// PubSubSourceMock -----------------------------------
type PubSubSourceMock struct {
	mock.Mock
}

func (pss *PubSubSourceMock) Subscribe(r *Route) *Route {
	args := pss.Called(r)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*Route)
}

func (pss *PubSubSourceMock) Unsubscribe(r *Route) {
	pss.Called(r)
}

// MessageSinkMock -----------------------------------
type MessageSinkMock struct {
	mock.Mock
}

func (sink *MessageSinkMock) HandleMessage(message Message) {
	sink.Called(message)
}

// WSConnMock -----------------------------------
type WSConnMock struct {
	mock.Mock
}

func (conn *WSConnMock) Close() {
	conn.Called()
}

func (conn *WSConnMock) LocationString() string {
	return conn.Called().String(0)
}

func (conn *WSConnMock) Send(bytes []byte) (err error) {
	return conn.Called(bytes).Error(0)
}

func (conn *WSConnMock) Receive(bytes *[]byte) (err error) {
	return conn.Called(bytes).Error(0)
}
