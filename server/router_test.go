package server

import (
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/auth"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var aTestByteMessage = []byte("Hello World!")
var chanSize = 10

func TestAddAndRemoveRoutes(t *testing.T) {
	a := assert.New(t)

	accessManager := auth.NewAllowAllAccessManager(true)

	// Given a Multiplexer
	router := NewRouter(accessManager, nil, nil).(*router)
	router.Start()

	// when i add two routes in the same path
	channel := make(chan MsgAndRoute, chanSize)
	routeBlah1, _ := router.Subscribe(NewRoute("/blah", channel, "appid01", "user01"))
	routeBlah2, _ := router.Subscribe(NewRoute("/blah", channel, "appid02", "user01"))

	// and one route in another path
	routeFoo, _ := router.Subscribe(NewRoute("/foo", channel, "appid01", "user01"))

	// then

	// the routes are stored
	a.Equal(2, len(router.routes[protocol.Path("/blah")]))
	a.True(routeBlah1.equals(router.routes[protocol.Path("/blah")][0]))
	a.True(routeBlah2.equals(router.routes[protocol.Path("/blah")][1]))

	a.Equal(1, len(router.routes[protocol.Path("/foo")]))
	a.True(routeFoo.equals(router.routes[protocol.Path("/foo")][0]))

	// when i remove routes
	router.Unsubscribe(routeBlah1)
	router.Unsubscribe(routeFoo)

	// then they are gone
	a.Equal(1, len(router.routes[protocol.Path("/blah")]))
	a.True(routeBlah2.equals(router.routes[protocol.Path("/blah")][0]))

	a.Nil(router.routes[protocol.Path("/foo")])
}

func Test_SubscribeNotAllowed(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	tam := NewMockAccessManager(ctrl)
	tam.EXPECT().IsAllowed(auth.READ, "user01", protocol.Path("/blah")).Return(false)

	router := NewRouter(tam, nil, nil).(*router)
	router.Start()

	channel := make(chan MsgAndRoute, chanSize)
	_, e := router.Subscribe(NewRoute("/blah", channel, "appid01", "user01"))

	// default TestAccessManager denies all
	a.NotNil(e)

	// now add permissions
	tam.EXPECT().IsAllowed(auth.READ, "user01", protocol.Path("/blah")).Return(true)

	// and user shall be allowed to subscribe
	_, e = router.Subscribe(NewRoute("/blah", channel, "appid01", "user01"))

	a.Nil(e)

}

func Test_HandleMessageNotAllowed(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	tam := NewMockAccessManager(ctrl)
	msMock := NewMockMessageStore(ctrl)

	// Given a Multiplexer with route
	router, r := aRouterRoute()
	router.accessManager = tam
	router.messageStore = msMock

	tam.EXPECT().IsAllowed(auth.WRITE, r.UserID, r.Path).Return(false)

	// when i send a message to the route
	e := router.HandleMessage(&protocol.Message{
		Path:   r.Path,
		Body:   aTestByteMessage,
		UserID: r.UserID,
	})

	// an error shall be returned
	a.NotNil(e)

	// and when permission is granted
	tam.EXPECT().IsAllowed(auth.WRITE, r.UserID, r.Path).Return(true)
	msMock.EXPECT().StoreTx(r.Path.Partition(), gomock.Any()).Return(nil)

	// sending message
	e = router.HandleMessage(&protocol.Message{
		Path:   r.Path,
		Body:   aTestByteMessage,
		UserID: r.UserID,
	})

	// shall give no error
	a.Nil(e)
}

func TestReplacingOfRoutes(t *testing.T) {
	a := assert.New(t)

	// Given a router with a route
	router := NewRouter(auth.NewAllowAllAccessManager(true), nil, nil).(*router)
	router.Start()

	router.Subscribe(NewRoute("/blah", nil, "appid01", "user01"))

	// when: i add another route with the same Application Id and Same Path
	router.Subscribe(NewRoute("/blah", nil, "appid01", "newUserId"))

	// then: the router only contains the new route
	a.Equal(1, len(router.routes))
	a.Equal(1, len(router.routes["/blah"]))
	a.Equal("newUserId", router.routes["/blah"][0].UserID)
}

func TestSimpleMessageSending(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	// Given a Multiplexer with route
	router, r := aRouterRoute()

	msMock := NewMockMessageStore(ctrl)
	router.messageStore = msMock
	msMock.EXPECT().StoreTx(r.Path.Partition(), gomock.Any()).Return(nil)

	// when i send a message to the route
	router.HandleMessage(&protocol.Message{Path: r.Path, Body: aTestByteMessage})

	// then I can receive it a short time later
	assertChannelContainsMessage(a, r.C, aTestByteMessage)
}

func TestRoutingWithSubTopics(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	// Given a Multiplexer with route
	router := NewRouter(auth.NewAllowAllAccessManager(true), nil, nil).(*router)
	router.Start()

	msMock := NewMockMessageStore(ctrl)
	router.messageStore = msMock
	// expect a message to `blah` partition first and `blahblub` second
	msMock.EXPECT().StoreTx("blah", gomock.Any()).Return(nil)
	msMock.EXPECT().StoreTx("blahblub", gomock.Any()).Return(nil)

	channel := make(chan MsgAndRoute, chanSize)
	r, _ := router.Subscribe(NewRoute("/blah", channel, "appid01", "user01"))

	// when i send a message to a subroute
	router.HandleMessage(&protocol.Message{Path: "/blah/blub", Body: aTestByteMessage})

	// then I can receive the message
	assertChannelContainsMessage(a, r.C, aTestByteMessage)

	// but, when i send a message to a resource, which is just a substring
	router.HandleMessage(&protocol.Message{Path: "/blahblub", Body: aTestByteMessage})

	// then the message gets not delivered
	a.Equal(0, len(r.C))
}

func TestMatchesTopic(t *testing.T) {
	for _, test := range []struct {
		messagePath protocol.Path
		routePath   protocol.Path
		matches     bool
	}{
		{"/foo", "/foo", true},
		{"/foo/xyz", "/foo", true},
		{"/foo", "/bar", false},
		{"/fooxyz", "/foo", false},
		{"/foo", "/bar/xyz", false},
	} {
		if !test.matches == matchesTopic(test.messagePath, test.routePath) {
			t.Errorf("error: expected %v, but: matchesTopic(%q, %q) = %v", test.matches, test.messagePath, test.routePath, matchesTopic(test.messagePath, test.routePath))
		}
	}
}

func TestRouteIsRemovedIfChannelIsFull(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	// Given a Multiplexer with route
	router, r := aRouterRoute()

	msMock := NewMockMessageStore(ctrl)
	router.messageStore = msMock
	msMock.EXPECT().StoreTx(gomock.Any(), gomock.Any()).MaxTimes(chanSize + 1)

	// where the channel is full of messages
	for i := 0; i < chanSize; i++ {
		router.HandleMessage(&protocol.Message{Path: r.Path, Body: aTestByteMessage})
	}

	// when I send one more message
	done := make(chan bool, 1)
	go func() {
		router.HandleMessage(&protocol.Message{Path: r.Path, Body: aTestByteMessage})
		done <- true
	}()

	// then: the it returns immediately
	select {
	case <-done:
	case <-time.After(time.Millisecond * 10):
		a.Fail("Not returning!")
	}

	time.Sleep(time.Millisecond * 1)

	// fetch messages from the channel
	for i := 0; i < chanSize; i++ {
		select {
		case _, open := <-r.C:
			a.True(open)
		case <-time.After(time.Millisecond * 10):
			a.Fail("error not enough messages in channel")
		}
	}

	// and the channel is closed
	select {
	case _, open := <-r.C:
		a.False(open)
	default:
		fmt.Printf("len(r.C): %v", len(r.C))
		a.Fail("channel was not closed")
	}
}

func Test_Router_storeInTxAndHandle(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	startTime := time.Now()

	msg := &protocol.Message{Path: protocol.Path("/topic1")}
	var storedMsg []byte

	messageStoreMock := NewMockMessageStore(ctrl)
	router := NewRouter(
		auth.NewAllowAllAccessManager(true),
		messageStoreMock,
		nil,
	).(*router)

	messageStoreMock.EXPECT().StoreTx("topic1", gomock.Any()).
		Do(func(topic string, callback func(msgId uint64) []byte) {
			storedMsg = callback(uint64(42))
		})

	receive := make(chan *protocol.Message)
	go func() {
		msg := <-router.messageIn

		a.Equal(uint64(42), msg.ID)
		t := time.Unix(msg.Time, 0) // publishing time
		a.True(t.After(startTime.Add(-1 * time.Second)))
		a.True(t.Before(time.Now().Add(time.Second)))

		receive <- msg
	}()
	router.HandleMessage(msg)
	routedMsg := <-receive

	a.Equal(routedMsg.Bytes(), storedMsg)
}

func aRouterRoute() (*router, *Route) {
	router := NewRouter(auth.NewAllowAllAccessManager(true), nil, nil).(*router)
	router.Start()
	route, _ := router.Subscribe(
		NewRoute("/blah", make(chan MsgAndRoute, chanSize), "appid01", "user01"),
	)
	return router, route
}

func assertChannelContainsMessage(a *assert.Assertions, c chan MsgAndRoute, msg []byte) {
	//log.Println("DEBUG: start assertChannelContainsMessage-> select")
	select {
	case msgBack := <-c:
		a.Equal(string(msg), string(msgBack.Message.Body))
	case <-time.After(time.Millisecond):
		a.Fail("No message received")
	}
}
