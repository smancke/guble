package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"fmt"
	"strings"
	"testing"
	"time"
)

var aTestMessage = &guble.Message{
	Id:   uint64(42),
	Path: "/foo",
	Body: []byte("Test"),
}

func Test_WSHandler_SubscribeAndUnsubscribe(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	messages := []string{"+ /foo", "+ /bar", "- /foo"}
	wsconn, pubSubSource, messageSink, messageStore := createDefaultMocks(messages)

	pubSubSource.EXPECT().Subscribe(routeMatcher{"/foo"}).Return(nil)
	wsconn.EXPECT().Send([]byte("#" + guble.SUCCESS_SUBSCRIBED_TO + " /foo"))

	pubSubSource.EXPECT().Subscribe(routeMatcher{"/bar"}).Return(nil)
	wsconn.EXPECT().Send([]byte("#" + guble.SUCCESS_SUBSCRIBED_TO + " /bar"))

	pubSubSource.EXPECT().Unsubscribe(routeMatcher{"/foo"})
	wsconn.EXPECT().Send([]byte("#" + guble.SUCCESS_CANCELED + " /foo"))

	wshandler := runNewWsHandler(wsconn, pubSubSource, messageSink, messageStore)

	a.Equal(1, len(wshandler.receiver))
	a.Equal(guble.Path("/bar"), wshandler.receiver[guble.Path("/bar")].path)
}

func TestSendMessageWirthPublisherMessageId(t *testing.T) {
	defer initCtrl(t)()

	// given: a send command with PublisherMessageId
	commands := []string{"> /path 42"}
	wsconn, pubSubSource, messageSink, messageStore := createDefaultMocks(commands)

	messageSink.EXPECT().HandleMessage(gomock.Any()).Do(func(msg *guble.Message) {
		assert.Equal(t, guble.Path("/path"), msg.Path)
		assert.Equal(t, "42", msg.PublisherMessageId)
	})

	wsconn.EXPECT().Send([]byte("#send 42"))

	runNewWsHandler(wsconn, pubSubSource, messageSink, messageStore)
}

func TestSendMessage(t *testing.T) {
	defer initCtrl(t)()

	commands := []string{"> /path\n{\"key\": \"value\"}\nHello, this is a test"}
	wsconn, pubSubSource, messageSink, messageStore := createDefaultMocks(commands)

	messageSink.EXPECT().HandleMessage(messageMatcher{path: "/path", message: "Hello, this is a test", header: `{"key": "value"}`})
	wsconn.EXPECT().Send([]byte("#send"))

	runNewWsHandler(wsconn, pubSubSource, messageSink, messageStore)
}

func TestAnIncommingMessageIsDelivered(t *testing.T) {
	defer initCtrl(t)()

	wsconn, pubSubSource, messageSink, messageStore := createDefaultMocks([]string{})

	wsconn.EXPECT().Send(aTestMessage.Bytes())

	handler := runNewWsHandler(wsconn, pubSubSource, messageSink, messageStore)

	handler.sendChannel <- aTestMessage.Bytes()
	time.Sleep(time.Millisecond * 2)
}

func TestBadCommands(t *testing.T) {
	defer initCtrl(t)()

	badRequests := []string{"XXXX", "", ">", ">/foo", "+", "-", "send /foo"}
	wsconn, pubSubSource, messageSink, messageStore := createDefaultMocks(badRequests)

	counter := 0

	wsconn.EXPECT().Send(gomock.Any()).Do(func(data []byte) error {
		if strings.HasPrefix(string(data), "#connected") {
			return nil
		}
		if strings.HasPrefix(string(data), "!error-bad-request") {
			counter++
		} else {
			t.Logf("expected bad-request, but got: %v", string(data))
		}
		return nil
	}).AnyTimes()

	runNewWsHandler(wsconn, pubSubSource, messageSink, messageStore)

	assert.Equal(t, len(badRequests), counter, "expected number of bad requests does not match")
}

func runNewWsHandler(wsconn *MockWSConn, pubSubSource *MockPubSubSource, messageSink *MockMessageSink, messageStore store.MessageStore) *WSHandler {
	handler := NewWSHandler(pubSubSource, messageSink, messageStore, wsconn, "testuser")
	go func() {
		handler.Start()
	}()
	time.Sleep(time.Millisecond * 2)
	return handler
}

func createDefaultMocks(inputMessages []string) (*MockWSConn, *MockPubSubSource, *MockMessageSink, *MockMessageStore) {
	inputMessagesC := make(chan []byte, 10)
	for _, msg := range inputMessages {
		inputMessagesC <- []byte(msg)
	}

	pubSubSource := NewMockPubSubSource(ctrl)
	messageSink := NewMockMessageSink(ctrl)
	messageStore := NewMockMessageStore(ctrl)

	wsconn := NewMockWSConn(ctrl)
	wsconn.EXPECT().Receive(gomock.Any()).Do(func(message *[]byte) error {
		*message = <-inputMessagesC
		return nil
	}).Times(len(inputMessages) + 1)

	wsconn.EXPECT().Send(connectedNotificationMatcher{})

	return wsconn, pubSubSource, messageSink, messageStore
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
	id      uint64
	path    string
	message string
	header  string
}

func (n messageMatcher) Matches(x interface{}) bool {
	return n.path == string(x.(*guble.Message).Path) &&
		n.message == string(x.(*guble.Message).Body) &&
		(n.id == 0 || n.id == x.(*guble.Message).Id) &&
		(n.header == "" || (n.header == x.(*guble.Message).HeaderJson))
}

func (n messageMatcher) String() string {
	return fmt.Sprintf("message equals %q, %q, %q", n.id, n.path, n.message)
}

// --- Connected Notification Matcher ---------
type connectedNotificationMatcher struct {
}

func (notify connectedNotificationMatcher) Matches(x interface{}) bool {
	return strings.HasPrefix(string(x.([]byte)), "#connected")
}

func (notify connectedNotificationMatcher) String() string {
	return fmt.Sprintf("is connected message")
}
