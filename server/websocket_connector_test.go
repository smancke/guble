package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"fmt"
	"github.com/smancke/guble/server/auth"
	"strings"
	"testing"
	"time"
)

var aTestMessage = &guble.Message{
	Id:   uint64(42),
	Path: "/foo",
	Body: []byte("Test"),
}

func Test_WebSocket_SubscribeAndUnsubscribe(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	messages := []string{"+ /foo", "+ /bar", "- /foo"}
	wsconn, routerMock, messageStore := createDefaultMocks(messages)

	routerMock.EXPECT().Subscribe(routeMatcher{"/foo"}).Return(nil, nil)
	wsconn.EXPECT().Send([]byte("#" + guble.SUCCESS_SUBSCRIBED_TO + " /foo"))

	routerMock.EXPECT().Subscribe(routeMatcher{"/bar"}).Return(nil, nil)
	wsconn.EXPECT().Send([]byte("#" + guble.SUCCESS_SUBSCRIBED_TO + " /bar"))

	routerMock.EXPECT().Unsubscribe(routeMatcher{"/foo"})
	wsconn.EXPECT().Send([]byte("#" + guble.SUCCESS_CANCELED + " /foo"))

	websocket := runNewWebSocket(wsconn, routerMock, messageStore, nil)

	a.Equal(1, len(websocket.receivers))
	a.Equal(guble.Path("/bar"), websocket.receivers[guble.Path("/bar")].path)
}

func Test_SendMessageWithPublisherMessageId(t *testing.T) {
	defer initCtrl(t)()

	// given: a send command with PublisherMessageId
	commands := []string{"> /path 42"}
	wsconn, routerMock, messageStore := createDefaultMocks(commands)

	routerMock.EXPECT().HandleMessage(gomock.Any()).Do(func(msg *guble.Message) {
		assert.Equal(t, guble.Path("/path"), msg.Path)
		assert.Equal(t, "42", msg.PublisherMessageId)
	})

	wsconn.EXPECT().Send([]byte("#send 42"))

	runNewWebSocket(wsconn, routerMock, messageStore, nil)
}

func Test_SendMessage(t *testing.T) {
	defer initCtrl(t)()

	commands := []string{"> /path\n{\"key\": \"value\"}\nHello, this is a test"}
	wsconn, routerMock, messageStore := createDefaultMocks(commands)

	routerMock.EXPECT().HandleMessage(messageMatcher{path: "/path", message: "Hello, this is a test", header: `{"key": "value"}`})
	wsconn.EXPECT().Send([]byte("#send"))

	runNewWebSocket(wsconn, routerMock, messageStore, nil)
}

func Test_AnIncommingMessageIsDelivered(t *testing.T) {
	defer initCtrl(t)()

	wsconn, routerMock, messageStore := createDefaultMocks([]string{})

	wsconn.EXPECT().Send(aTestMessage.Bytes())

	handler := runNewWebSocket(wsconn, routerMock, messageStore, nil)

	handler.sendChannel <- aTestMessage.Bytes()
	time.Sleep(time.Millisecond * 2)
}

func Test_AnIncommingMessageIsNotAllowed(t *testing.T) {
	defer initCtrl(t)()

	wsconn, routerMock, messageStore := createDefaultMocks([]string{})

	tam := NewMockAccessManager(ctrl)
	tam.EXPECT().IsAllowed(auth.READ, "testuser", guble.Path("/foo")).Return(false)
	handler := NewWebSocket(
		testWSHandler(routerMock, messageStore, tam),
		wsconn,
		"testuser",
	)
	go func() {
		handler.Start()
	}()
	time.Sleep(time.Millisecond * 2)

	handler.sendChannel <- aTestMessage.Bytes()
	time.Sleep(time.Millisecond * 2)
	//nothing shall have been sent

	//now allow
	tam.EXPECT().IsAllowed(auth.READ, "testuser", guble.Path("/foo")).Return(true)

	wsconn.EXPECT().Send(aTestMessage.Bytes())

	time.Sleep(time.Millisecond * 2)

	handler.sendChannel <- aTestMessage.Bytes()
	time.Sleep(time.Millisecond * 2)

}

func Test_BadCommands(t *testing.T) {
	defer initCtrl(t)()

	badRequests := []string{"XXXX", "", ">", ">/foo", "+", "-", "send /foo"}
	wsconn, routerMock, messageStore := createDefaultMocks(badRequests)

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

	runNewWebSocket(wsconn, routerMock, messageStore, nil)

	assert.Equal(t, len(badRequests), counter, "expected number of bad requests does not match")
}

func testWSHandler(
	routerMock *MockRouter,
	messageStore store.MessageStore,
	accessManager auth.AccessManager) *WSHandler {

	return &WSHandler{
		Router:        routerMock,
		prefix:        "/prefix",
		messageStore:  messageStore,
		accessManager: accessManager,
	}
}

func runNewWebSocket(
	wsconn *MockWSConnection,
	routerMock *MockRouter,
	messageStore store.MessageStore,
	accessManager auth.AccessManager) *WebSocket {

	if accessManager == nil {
		accessManager = auth.NewAllowAllAccessManager(true)
	}
	websocket := NewWebSocket(
		testWSHandler(routerMock, messageStore, accessManager),
		wsconn,
		"testuser",
	)

	go func() {
		websocket.Start()
	}()

	time.Sleep(time.Millisecond * 2)
	return websocket
}

func createDefaultMocks(inputMessages []string) (
	*MockWSConnection,
	*MockRouter,
	*MockMessageStore) {
	inputMessagesC := make(chan []byte, 10)
	for _, msg := range inputMessages {
		inputMessagesC <- []byte(msg)
	}

	routerMock := NewMockRouter(ctrl)
	messageStore := NewMockMessageStore(ctrl)

	wsconn := NewMockWSConnection(ctrl)
	wsconn.EXPECT().Receive(gomock.Any()).Do(func(message *[]byte) error {
		*message = <-inputMessagesC
		return nil
	}).Times(len(inputMessages) + 1)

	wsconn.EXPECT().Send(connectedNotificationMatcher{})

	return wsconn, routerMock, messageStore
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
		(n.header == "" || (n.header == x.(*guble.Message).HeaderJSON))
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
