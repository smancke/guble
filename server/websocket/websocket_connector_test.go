package websocket

import (
	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/auth"
	"github.com/cosminrentea/gobbler/server/router"
	"github.com/cosminrentea/gobbler/server/store"
	"github.com/cosminrentea/gobbler/testutil"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

var aTestMessage = &protocol.Message{
	ID:   uint64(42),
	Path: "/foo",
	Body: []byte("Test"),
}

func Test_WebSocket_SubscribeAndUnsubscribe(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	messages := []string{"+ /foo", "+ /bar", "- /foo"}
	wsconn, routerMock, messageStore := createDefaultMocks(messages)

	var wg sync.WaitGroup
	wg.Add(3)
	doneGroup := func(bytes []byte) error {
		wg.Done()
		return nil
	}

	routerMock.EXPECT().Subscribe(routeMatcher{"/foo"}).Return(nil, nil)
	wsconn.EXPECT().
		Send([]byte("#" + protocol.SUCCESS_SUBSCRIBED_TO + " /foo")).
		Do(doneGroup)

	routerMock.EXPECT().Subscribe(routeMatcher{"/bar"}).Return(nil, nil)
	wsconn.EXPECT().
		Send([]byte("#" + protocol.SUCCESS_SUBSCRIBED_TO + " /bar")).
		Do(doneGroup)

	routerMock.EXPECT().Unsubscribe(routeMatcher{"/foo"})
	wsconn.EXPECT().
		Send([]byte("#" + protocol.SUCCESS_CANCELED + " /foo")).
		Do(doneGroup)

	websocket := runNewWebSocket(wsconn, routerMock, messageStore, nil)
	wg.Wait()

	a.Equal(1, len(websocket.receivers))
	a.Equal(protocol.Path("/bar"), websocket.receivers[protocol.Path("/bar")].path)
}

func Test_SendMessage(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	commands := []string{"> /path\n{\"key\": \"value\"}\nHello, this is a test"}
	wsconn, routerMock, messageStore := createDefaultMocks(commands)

	routerMock.EXPECT().HandleMessage(messageMatcher{path: "/path", message: "Hello, this is a test", header: `{"key": "value"}`})
	wsconn.EXPECT().Send([]byte("#send"))

	runNewWebSocket(wsconn, routerMock, messageStore, nil)
}

func Test_AnIncomingMessageIsDelivered(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	wsconn, routerMock, messageStore := createDefaultMocks([]string{})

	wsconn.EXPECT().Send(aTestMessage.Bytes())

	handler := runNewWebSocket(wsconn, routerMock, messageStore, nil)

	handler.sendChannel <- aTestMessage.Bytes()
	time.Sleep(time.Millisecond * 2)
}

func Test_AnIncomingMessageIsNotAllowed(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	wsconn, routerMock, _ := createDefaultMocks([]string{})

	tam := NewMockAccessManager(ctrl)
	tam.EXPECT().IsAllowed(auth.READ, "testuser", protocol.Path("/foo")).Return(false)
	handler := NewWebSocket(
		testWSHandler(routerMock, tam),
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
	tam.EXPECT().IsAllowed(auth.READ, "testuser", protocol.Path("/foo")).Return(true)

	wsconn.EXPECT().Send(aTestMessage.Bytes())

	time.Sleep(time.Millisecond * 2)

	handler.sendChannel <- aTestMessage.Bytes()
	time.Sleep(time.Millisecond * 2)
}

func Test_BadCommands(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	badRequests := []string{"XXXX", "", ">", ">/foo", "+", "-", "send /foo"}
	wsconn, routerMock, messageStore := createDefaultMocks(badRequests)

	counter := 0

	var wg sync.WaitGroup
	wg.Add(len(badRequests))

	wsconn.EXPECT().Send(gomock.Any()).Do(func(data []byte) error {
		if strings.HasPrefix(string(data), "#connected") {
			return nil
		}
		if strings.HasPrefix(string(data), "!error-bad-request") {
			counter++
		} else {
			t.Logf("expected bad-request, but got: %v", string(data))
		}

		wg.Done()
		return nil
	}).AnyTimes()

	runNewWebSocket(wsconn, routerMock, messageStore, nil)

	wg.Wait()
	assert.Equal(t, len(badRequests), counter, "expected number of bad requests does not match")
}

func TestExtractUserId(t *testing.T) {
	assert.Equal(t, "marvin", extractUserID("/foo/user/marvin"))
	assert.Equal(t, "marvin", extractUserID("/user/marvin"))
	assert.Equal(t, "", extractUserID("/"))
}

func testWSHandler(
	routerMock *MockRouter,
	accessManager auth.AccessManager) *WSHandler {

	return &WSHandler{
		router:        routerMock,
		prefix:        "/prefix",
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
		testWSHandler(routerMock, accessManager),
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
	inputMessagesC := make(chan []byte, len(inputMessages))
	for _, msg := range inputMessages {
		inputMessagesC <- []byte(msg)
	}

	routerMock := NewMockRouter(testutil.MockCtrl)
	messageStore := NewMockMessageStore(testutil.MockCtrl)
	routerMock.EXPECT().MessageStore().Return(messageStore, nil).AnyTimes()

	wsconn := NewMockWSConnection(testutil.MockCtrl)
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
	return n.path == string(x.(*router.Route).Path)
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
	return n.path == string(x.(*protocol.Message).Path) &&
		n.message == string(x.(*protocol.Message).Body) &&
		(n.id == 0 || n.id == x.(*protocol.Message).ID) &&
		(n.header == "" || (n.header == x.(*protocol.Message).HeaderJSON))
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
