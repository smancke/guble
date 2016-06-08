package server

import (
	"errors"
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/auth"
	"github.com/smancke/guble/store"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var aTestByteMessage = []byte("Hello World!")
var chanSize = 10

func TestRouter_AddAndRemoveRoutes(t *testing.T) {
	a := assert.New(t)

	// Given a Router
	router, _, _, _ := aStartedRouter()

	// when i add two routes in the same path
	routeBlah1, _ := router.Subscribe(NewRoute("/blah", "appid01", "user01", chanSize))
	routeBlah2, _ := router.Subscribe(NewRoute("/blah", "appid02", "user01", chanSize))

	// and one route in another path
	routeFoo, _ := router.Subscribe(NewRoute("/foo", "appid01", "user01", chanSize))

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

func TestRouter_SubscribeNotAllowed(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	am := NewMockAccessManager(ctrl)
	msMock := NewMockMessageStore(ctrl)
	kvsMock := NewMockKVStore(ctrl)

	am.EXPECT().IsAllowed(auth.READ, "user01", protocol.Path("/blah")).Return(false)

	router := NewRouter(am, msMock, kvsMock).(*router)
	router.Start()

	_, e := router.Subscribe(NewRoute("/blah", "appid01", "user01", chanSize))

	// default TestAccessManager denies all
	a.NotNil(e)

	// now add permissions
	am.EXPECT().IsAllowed(auth.READ, "user01", protocol.Path("/blah")).Return(true)

	// and user shall be allowed to subscribe
	_, e = router.Subscribe(NewRoute("/blah", "appid01", "user01", chanSize))

	a.Nil(e)
}

func TestRouter_HandleMessageNotAllowed(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	amMock := NewMockAccessManager(ctrl)
	msMock := NewMockMessageStore(ctrl)
	kvsMock := NewMockKVStore(ctrl)

	// Given a Router with route
	router, r := aRouterRoute(chanSize)
	router.accessManager = amMock
	router.messageStore = msMock
	router.kvStore = kvsMock

	amMock.EXPECT().IsAllowed(auth.WRITE, r.UserID, r.Path).Return(false)

	// when i send a message to the route
	e := router.HandleMessage(&protocol.Message{
		Path:   r.Path,
		Body:   aTestByteMessage,
		UserID: r.UserID,
	})

	// an error shall be returned
	a.NotNil(e)

	// and when permission is granted
	amMock.EXPECT().IsAllowed(auth.WRITE, r.UserID, r.Path).Return(true)
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

func TestRouter_ReplacingOfRoutes(t *testing.T) {
	a := assert.New(t)

	// Given a Router with a route
	router, _, _, _ := aStartedRouter()

	router.Subscribe(NewRoute("/blah", "appid01", "user01", 0))

	// when: i add another route with the same Application Id and Same Path
	router.Subscribe(NewRoute("/blah", "appid01", "newUserId", 0))

	// then: the router only contains the new route
	a.Equal(1, len(router.routes))
	a.Equal(1, len(router.routes["/blah"]))
	a.Equal("newUserId", router.routes["/blah"][0].UserID)
}

func TestRouter_SimpleMessageSending(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	// Given a Router with route
	router, r := aRouterRoute(chanSize)
	msMock := NewMockMessageStore(ctrl)
	router.messageStore = msMock
	msMock.EXPECT().StoreTx(r.Path.Partition(), gomock.Any()).Return(nil)

	// when i send a message to the route
	router.HandleMessage(&protocol.Message{Path: r.Path, Body: aTestByteMessage})

	// then I can receive it a short time later
	assertChannelContainsMessage(a, r.MessagesChannel(), aTestByteMessage)
}

func TestRouter_RoutingWithSubTopics(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	// Given a Router with route
	router, _, _, _ := aStartedRouter()

	msMock := NewMockMessageStore(ctrl)
	router.messageStore = msMock
	// expect a message to `blah` partition first and `blahblub` second
	msMock.EXPECT().StoreTx("blah", gomock.Any()).Return(nil)
	msMock.EXPECT().StoreTx("blahblub", gomock.Any()).Return(nil)

	r, _ := router.Subscribe(NewRoute("/blah", "appid01", "user01", chanSize))

	// when i send a message to a subroute
	router.HandleMessage(&protocol.Message{Path: "/blah/blub", Body: aTestByteMessage})

	// then I can receive the message
	assertChannelContainsMessage(a, r.MessagesChannel(), aTestByteMessage)

	// but, when i send a message to a resource, which is just a substring
	router.HandleMessage(&protocol.Message{Path: "/blahblub", Body: aTestByteMessage})

	// then the message gets not delivered
	a.Equal(0, len(r.MessagesChannel()))
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
			t.Errorf("error: expected %v, but: matchesTopic(%q, %q) = %v",
				test.matches, test.messagePath, test.routePath, matchesTopic(test.messagePath, test.routePath))
		}
	}
}

func TestRoute_IsRemovedIfChannelIsFull(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	// Given a Router with route
	router, r := aRouterRoute(chanSize)

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

	// then: it returns immediately
	select {
	case <-done:
	case <-time.After(time.Millisecond * 10):
		a.Fail("Not returning!")
	}

	time.Sleep(time.Millisecond)

	// fetch messages from the channel
	for i := 0; i < chanSize; i++ {
		select {
		case _, open := <-r.MessagesChannel():
			a.True(open)
		case <-time.After(time.Millisecond * 10):
			a.Fail("error not enough messages in channel")
		}
	}

	// and the channel is closed
	select {
	case _, open := <-r.MessagesChannel():
		a.False(open)
	default:
		fmt.Printf("len(r.C): %v", len(r.MessagesChannel()))
		a.Fail("channel was not closed")
	}
}

func TestRouter_storeInTxAndHandle(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)
	startTime := time.Now()

	msg := &protocol.Message{Path: protocol.Path("/topic1")}
	var storedMsg []byte

	am := auth.NewAllowAllAccessManager(true)
	msMock := NewMockMessageStore(ctrl)
	router := NewRouter(am, msMock, nil).(*router)

	msMock.EXPECT().StoreTx("topic1", gomock.Any()).
		Do(func(topic string, callback func(msgId uint64) []byte) {
			storedMsg = callback(uint64(42))
		})

	receive := make(chan *protocol.Message)
	go func() {
		msg := <-router.handleC

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

// Router should handle the buffered messages also after the closing of the route
func TestRouter_CleanShutdown(t *testing.T) {
	//testutil.EnableDebugForMethod()
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	assert := assert.New(t)

	var ID uint64

	msMock := NewMockMessageStore(ctrl)
	msMock.EXPECT().StoreTx("blah", gomock.Any()).
		Return(nil).
		Do(func(partition string, callback func(msgID uint64) []byte) error {
			ID++
			callback(ID)
			return nil
		}).
		AnyTimes()

	router, _, _, _ := aStartedRouter()
	router.messageStore = msMock

	route, err := router.Subscribe(NewRoute("/blah", "appid01", "user01", 3))
	assert.Nil(err)

	doneC := make(chan bool)

	// read the messages until done is closed
	go func() {
		for {
			_, ok := <-route.MessagesChannel()
			select {
			case <-doneC:
				return
			default:
				assert.True(ok)
			}
		}
	}()

	// Send messages in the router until error
	go func() {
		for {
			err := router.HandleMessage(&protocol.Message{
				Path: protocol.Path("/blah"),
				Body: aTestByteMessage,
			})

			if err != nil {
				mse, ok := err.(*ModuleStoppingError)
				assert.True(ok)
				assert.Equal("Router", mse.Name)
				return
			}

			// if doneC channel has been closed and no error then we must fail the test
			select {
			case _, ok := <-doneC:
				if !ok {
					assert.Fail("Expected error from router handle message")
				}
			default:
			}
		}
	}()

	close(doneC)
	err = router.Stop()
	assert.Nil(err)

	// wait for above goroutine to finish
	<-time.After(50 * time.Millisecond)
}

func TestRouter_Check(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	amMock := NewMockAccessManager(ctrl)
	msMock := NewMockMessageStore(ctrl)
	kvsMock := NewMockKVStore(ctrl)

	// Given a Multiplexer with route
	router, _, _, _ := aStartedRouter()

	// Test 0: Router is healthy by default
	a.Nil(router.Check())

	// Test 1a: Given accessManager is nil, then router's Check returns error
	router.accessManager = nil
	router.messageStore = msMock
	router.kvStore = kvsMock
	a.NotNil(router.Check())

	// Test 1b: Given messageStore is nil, then router's Check returns error
	router.accessManager = amMock
	router.messageStore = nil
	router.kvStore = kvsMock
	a.NotNil(router.Check())

	// Test 1c: Given kvStore is nil, then router's Check return error
	router.accessManager = amMock
	router.messageStore = msMock
	router.kvStore = nil
	a.NotNil(router.Check())

	// Test 2: Given mocked store dependencies, both healthy
	router.accessManager = amMock
	router.messageStore = msMock
	router.kvStore = kvsMock

	msMock.EXPECT().Check().Return(nil)
	kvsMock.EXPECT().Check().Return(nil)

	// Then the aggregated router health check will return "no error" / nil
	a.Nil(router.Check())

	// Test 3: Given a mocked messageStore which returns error on Check(),
	// Then router's aggregated Check() should return error
	msMock.EXPECT().Check().Return(errors.New("HDD Disk is almost full."))
	a.NotNil(router.Check())

	// Test 4: Given a mocked kvStore which returns an error on Check()
	// and a healthy messageStore,
	// Then router's aggregated Check should return error
	kvsMock.EXPECT().Check().Return(errors.New("DB closed"))
	msMock.EXPECT().Check().Return(nil)
	a.NotNil(router.Check())
}

func aStartedRouter() (*router, auth.AccessManager, store.MessageStore, store.KVStore) {
	am := auth.NewAllowAllAccessManager(true)
	kvs := store.NewMemoryKVStore()
	ms := store.NewDummyMessageStore(kvs)
	router := NewRouter(am, ms, kvs).(*router)
	router.Start()
	return router, am, ms, kvs
}

func aRouterRoute(chSize int) (*router, *Route) {
	router, _, _, _ := aStartedRouter()
	route, _ := router.Subscribe(
		NewRoute("/blah", "appid01", "user01", chanSize),
	)
	return router, route
}

func assertChannelContainsMessage(a *assert.Assertions, c chan *protocol.Message, msg []byte) {
	select {
	case m := <-c:
		a.Equal(string(msg), string(m.Body))
	case <-time.After(time.Millisecond * 5):
		a.Fail("No message received")
	}
}
