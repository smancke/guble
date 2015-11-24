package server

import (
	"github.com/golang/mock/gomock"
	_ "github.com/stretchr/testify/assert"
	"testing"
	"time"

	guble "github.com/smancke/guble/guble"
)

var ctrl *gomock.Controller

func TestSubscriptionMessage(t *testing.T) {
	defer initCtrl(t)()

	messages := []string{"subscribe /mock", "subscribe /foo"}
	wsconn, pubSubSource, messageSink := createDefaultMocks(messages)

	pubSubSource.EXPECT().Subscribe(routeMatcher{"/mock"}).Return(nil)
	wsconn.EXPECT().Send([]byte("subscribed to /mock\n"))
	pubSubSource.EXPECT().Subscribe(routeMatcher{"/foo"}).Return(nil)
	wsconn.EXPECT().Send([]byte("subscribed to /foo\n"))

	runNewWsHandler(wsconn, pubSubSource, messageSink)
}

func TestSendMessage(t *testing.T) {
	defer initCtrl(t)()

	messages := []string{"send /path Hello, this is a test"}
	wsconn, pubSubSource, messageSink := createDefaultMocks(messages)

	messageSink.EXPECT().HandleMessage(messageMatcher{"/path", "Hello, this is a test"})
	wsconn.EXPECT().Send([]byte("sent message.\n"))

	runNewWsHandler(wsconn, pubSubSource, messageSink)
}

func initCtrl(t *testing.T) func() {
	ctrl = gomock.NewController(t)
	return func() { ctrl.Finish() }
}

func runNewWsHandler(wsconn *MockWSConn, pubSubSource *MockPubSubSource, messageSink *MockMessageSink) {
	handler := NewWSHandler(pubSubSource, messageSink)
	go func() {
		handler.HandleNewConnection(wsconn)
	}()
	time.Sleep(time.Millisecond * 10)
}

func createDefaultMocks(inputMessages []string) (*MockWSConn, *MockPubSubSource, *MockMessageSink) {
	inputMessagesC := make(chan []byte, 10)
	for _, msg := range inputMessages {
		inputMessagesC <- []byte(msg)
	}

	pubSubSource := NewMockPubSubSource(ctrl)
	messageSink := NewMockMessageSink(ctrl)

	wsconn := NewMockWSConn(ctrl)
	wsconn.EXPECT().Receive(gomock.Any()).Do(func(message *[]byte) error {
		*message = <-inputMessagesC
		return nil
	}).Times(len(inputMessages) + 1)

	return wsconn, pubSubSource, messageSink
}

// --- routeMatcher ---------
type routeMatcher struct {
	path string
}

func (n routeMatcher) Matches(x interface{}) bool {
	return n.path == string(x.(*Route).Path)
}

func (n routeMatcher) String() string {
	return "route path equals " + n.path
}

// --- messageMatcher ---------
type messageMatcher struct {
	path    string
	message string
}

func (n messageMatcher) Matches(x interface{}) bool {
	return n.path == string(x.(guble.Message).Path) &&
		n.message == string(x.(guble.Message).Body)
}

func (n messageMatcher) String() string {
	return "message equals " + n.path + " " + n.message
}
