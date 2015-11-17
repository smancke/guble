package server

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

var pubSubSourceMock *PubSubSourceMock
var messageSinkMock *MessageSinkMock

func TestSubscriptionMessage(t *testing.T) {
	a := assert.New(t)

	// Given
	handler := aMockedWSHandler()
	subscriptionPath := "/bla"

	// when I
	sendASubscribeMessage(handler, subscriptionPath)

	// then the mock was called for subscription
	// TODO
	a.Equal(16, 16)
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
