package server

import (
	"github.com/golang/mock/gomock"
	_ "github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var ctrl *gomock.Controller

func TestSubscriptionMessage(t *testing.T) {
	defer initCtrl(t)()

	messages := []string{"hallo"} //, "foo"}
	wsconn, pubSubSource, messageSink := createDefaultMocks(messages)

	wsconn.EXPECT().LocationString().Return("/mock")
	pubSubSource.EXPECT().Subscribe(routeMatcher{"/mock"}).Return(nil)
	pubSubSource.EXPECT().Subscribe(routeMatcher{"/foo"}).Return(nil)

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
		inputMessage := <-inputMessagesC
		message = &inputMessage
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
