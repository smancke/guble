package router

import (
	"errors"
	"testing"
	"time"

	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/auth"
	"github.com/cosminrentea/gobbler/server/kvstore"
	"github.com/cosminrentea/gobbler/server/store"
	"github.com/cosminrentea/gobbler/server/store/dummystore"
	"github.com/cosminrentea/gobbler/testutil"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

var aTestByteMessage = []byte("Hello World!")

type msChecker struct {
	*MockMessageStore
	*MockChecker
}

func newMSChecker() *msChecker {
	return &msChecker{
		NewMockMessageStore(testutil.MockCtrl),
		NewMockChecker(testutil.MockCtrl),
	}
}

type kvsChecker struct {
	*MockKVStore
	*MockChecker
}

func newKVSChecker() *kvsChecker {
	return &kvsChecker{
		NewMockKVStore(testutil.MockCtrl),
		NewMockChecker(testutil.MockCtrl),
	}
}

func TestRouter_AddAndRemoveRoutes(t *testing.T) {
	a := assert.New(t)

	// Given a Router
	router, _, _, _ := aStartedRouter()

	// when i add two routes in the same path
	routeBlah1, _ := router.Subscribe(NewRoute(
		RouteConfig{
			RouteParams: RouteParams{"application_id": "appid01", "user_id": "user01"},
			Path:        protocol.Path("/blah"),
			ChannelSize: chanSize,
		},
	))
	routeBlah2, _ := router.Subscribe(NewRoute(
		RouteConfig{
			RouteParams: RouteParams{"application_id": "appid02", "user_id": "user01"},
			Path:        protocol.Path("/blah"),
			ChannelSize: chanSize,
		},
	))

	// and one route in another path
	routeFoo, _ := router.Subscribe(NewRoute(
		RouteConfig{
			RouteParams: RouteParams{"application_id": "appid01", "user_id": "user01"},
			Path:        protocol.Path("/foo"),
			ChannelSize: chanSize,
		},
	))

	// then

	// the routes are stored
	a.Equal(2, len(router.routes[protocol.Path("/blah")]))
	a.True(routeBlah1.Equal(router.routes[protocol.Path("/blah")][0]))
	a.True(routeBlah2.Equal(router.routes[protocol.Path("/blah")][1]))

	a.Equal(1, len(router.routes[protocol.Path("/foo")]))
	a.True(routeFoo.Equal(router.routes[protocol.Path("/foo")][0]))

	// when i remove routes
	router.Unsubscribe(routeBlah1)
	router.Unsubscribe(routeFoo)

	// then they are gone
	a.Equal(1, len(router.routes[protocol.Path("/blah")]))
	a.True(routeBlah2.Equal(router.routes[protocol.Path("/blah")][0]))

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

	router := New(am, msMock, kvsMock, nil).(*router)
	router.Start()

	_, e := router.Subscribe(NewRoute(
		RouteConfig{
			RouteParams: RouteParams{"application_id": "appid01", "user_id": "user01"},
			Path:        protocol.Path("/blah"),
			ChannelSize: chanSize,
		},
	))

	// default TestAccessManager denies all
	a.NotNil(e)

	// now add permissions
	am.EXPECT().IsAllowed(auth.READ, "user01", protocol.Path("/blah")).Return(true)

	// and user shall be allowed to subscribe
	_, e = router.Subscribe(NewRoute(
		RouteConfig{
			RouteParams: RouteParams{"application_id": "appid01", "user_id": "user01"},
			Path:        protocol.Path("/blah"),
			ChannelSize: chanSize,
		},
	))

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

	amMock.EXPECT().IsAllowed(auth.WRITE, r.Get("user_id"), r.Path).Return(false)

	// when i send a message to the route
	err := router.HandleMessage(&protocol.Message{
		Path:   r.Path,
		Body:   aTestByteMessage,
		UserID: r.Get("user_id"),
	})

	// an error shall be returned
	a.Error(err)

	// and when permission is granted
	id, ts := uint64(2), time.Now().Unix()

	amMock.EXPECT().IsAllowed(auth.WRITE, r.Get("user_id"), r.Path).Return(true)
	msMock.EXPECT().
		StoreMessage(gomock.Any(), gomock.Any()).
		Do(func(m *protocol.Message, nodeID uint8) (int, error) {
			m.ID = id
			m.Time = ts
			m.NodeID = nodeID
			return len(m.Bytes()), nil
		})

	// sending message
	err = router.HandleMessage(&protocol.Message{
		Path:   r.Path,
		Body:   aTestByteMessage,
		UserID: r.Get("user_id"),
	})

	// shall give no error
	a.NoError(err)
}

func TestRouter_ReplacingOfRoutesMatchingAppID(t *testing.T) {
	a := assert.New(t)

	// Given a Router with a route
	router, _, _, _ := aStartedRouter()

	matcherFunc := func(route, other RouteConfig, keys ...string) bool {
		return route.Path == other.Path && route.Get("application_id") == other.Get("application_id")
	}
	router.Subscribe(NewRoute(
		RouteConfig{
			RouteParams: RouteParams{"application_id": "appid01", "user_id": "user01"},
			Path:        protocol.Path("/blah"),
			Matcher:     matcherFunc,
		},
	))

	// when: i add another route with the same Application Id and Same Path
	router.Subscribe(NewRoute(
		RouteConfig{
			RouteParams: RouteParams{"application_id": "appid01", "user_id": "newUserId"},
			Path:        protocol.Path("/blah"),
			Matcher:     matcherFunc,
		},
	))

	// then: the router only contains the new route
	a.Equal(1, len(router.routes))
	a.Equal(1, len(router.routes["/blah"]))
	a.Equal("newUserId", router.routes["/blah"][0].Get("user_id"))
}

func TestRouter_SimpleMessageSending(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	// Given a Router with route
	router, r := aRouterRoute(chanSize)
	msMock := NewMockMessageStore(ctrl)
	router.messageStore = msMock

	id, ts := uint64(2), time.Now().Unix()
	msMock.EXPECT().
		StoreMessage(gomock.Any(), gomock.Any()).
		Do(func(m *protocol.Message, nodeID uint8) (int, error) {
			m.ID = id
			m.Time = ts
			m.NodeID = nodeID
			return len(m.Bytes()), nil
		})

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
	firstStore := msMock.EXPECT().
		StoreMessage(gomock.Any(), gomock.Any()).
		Do(func(m *protocol.Message, nodeID uint8) (int, error) {
			a.Equal("/blah/blub", string(m.Path))
			return 0, nil
		})

	msMock.EXPECT().
		StoreMessage(gomock.Any(), gomock.Any()).After(firstStore).
		Do(func(m *protocol.Message, nodeID uint8) (int, error) {
			a.Equal("/blahblub", string(m.Path))
			return 0, nil
		})

	r, _ := router.Subscribe(NewRoute(
		RouteConfig{
			RouteParams: RouteParams{"application_id": "appid01", "user_id": "user01"},
			Path:        protocol.Path("/blah"),
			ChannelSize: chanSize,
		},
	))

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
	r.Timeout = 5 * time.Millisecond

	msMock := NewMockMessageStore(ctrl)
	router.messageStore = msMock

	msMock.EXPECT().
		StoreMessage(gomock.Any(), gomock.Any()).
		Do(func(m *protocol.Message, nodeID uint8) (int, error) {
			a.Equal(r.Path, m.Path)
			return 0, nil
		}).MaxTimes(chanSize + 1)

	// where the channel is full of messages
	for i := 0; i < chanSize; i++ {
		router.HandleMessage(&protocol.Message{Path: r.Path, Body: aTestByteMessage})
	}

	// when I send one more message
	done := make(chan bool)
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
		logger.Debug("len(r.C): %v", len(r.MessagesChannel()))
		a.Fail("channel was not closed")
	}
}

// Router should handle the buffered messages also after the closing of the route
func TestRouter_CleanShutdown(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	var ID uint64

	msMock := NewMockMessageStore(ctrl)
	msMock.EXPECT().Store("blah", gomock.Any(), gomock.Any()).
		Return(nil).
		Do(func(partition string, callback func(msgID uint64) []byte) error {
			ID++
			callback(ID)
			return nil
		}).
		AnyTimes()

	router, _, _, _ := aStartedRouter()
	router.messageStore = msMock

	route, err := router.Subscribe(NewRoute(
		RouteConfig{
			RouteParams: RouteParams{"application_id": "appid01", "user_id": "user01"},
			Path:        protocol.Path("/blah"),
			ChannelSize: 3,
		},
	))
	a.Nil(err)

	doneC := make(chan bool)

	// read the messages until done is closed
	go func() {
		for {
			_, ok := <-route.MessagesChannel()
			select {
			case <-doneC:
				return
			default:
				a.True(ok)
			}
		}
	}()

	// Send messages in the router until error
	go func() {
		for {
			errHandle := router.HandleMessage(&protocol.Message{
				Path: protocol.Path("/blah"),
				Body: aTestByteMessage,
			})

			if errHandle != nil {
				mse, ok := errHandle.(*ModuleStoppingError)
				a.True(ok)
				a.Equal("Router", mse.Name)
				return
			}

			// if doneC channel has been closed and no error then we must fail the test
			select {
			case _, ok := <-doneC:
				if !ok {
					a.Fail("Expected error from router handle message")
				}
			default:
			}
		}
	}()

	close(doneC)
	err = router.Stop()
	a.Nil(err)

	// wait for above goroutine to finish
	<-time.After(100 * time.Millisecond)
}

func TestRouter_Check(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	amMock := NewMockAccessManager(ctrl)
	msMock := NewMockMessageStore(ctrl)
	kvsMock := NewMockKVStore(ctrl)
	msCheckerMock := newMSChecker()
	kvsCheckerMock := newKVSChecker()

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
	router.messageStore = msCheckerMock
	router.kvStore = kvsCheckerMock

	msCheckerMock.MockChecker.EXPECT().Check().Return(nil)
	kvsCheckerMock.MockChecker.EXPECT().Check().Return(nil)

	// Then the aggregated router health check will return "no error" / nil
	a.Nil(router.Check())

	// Test 3: Given a mocked messageStore which returns error on Check(),
	// Then router's aggregated Check() should return error
	msCheckerMock.MockChecker.EXPECT().Check().Return(errors.New("Storage is almost full"))
	a.NotNil(router.Check())

	// Test 4: Given a mocked kvStore which returns an error on Check()
	// and a healthy messageStore,
	// Then router's aggregated Check should return error
	msCheckerMock.MockChecker.EXPECT().Check().Return(nil)
	kvsCheckerMock.MockChecker.EXPECT().Check().Return(errors.New("DB closed"))
	a.NotNil(router.Check())
}

func TestPanicOnInternalDependencies(t *testing.T) {
	defer testutil.ExpectPanic(t)
	router := New(nil, nil, nil, nil).(*router)
	router.panicIfInternalDependenciesAreNil()
}

func aStartedRouter() (*router, auth.AccessManager, store.MessageStore, kvstore.KVStore) {
	am := auth.NewAllowAllAccessManager(true)
	kvs := kvstore.NewMemoryKVStore()
	ms := dummystore.New(kvs)
	router := New(am, ms, kvs, nil).(*router)
	router.Start()
	return router, am, ms, kvs
}

func aRouterRoute(unused int) (*router, *Route) {
	router, _, _, _ := aStartedRouter()
	route, _ := router.Subscribe(NewRoute(
		RouteConfig{
			RouteParams: RouteParams{"application_id": "appid01", "user_id": "user01"},
			Path:        protocol.Path("/blah"),
			ChannelSize: chanSize,
		},
	))
	return router, route
}

func assertChannelContainsMessage(a *assert.Assertions, c <-chan *protocol.Message, msg []byte) {
	select {
	case m := <-c:
		a.Equal(string(msg), string(m.Body))
	case <-time.After(time.Millisecond * 5):
		a.Fail("No message received")
	}
}
