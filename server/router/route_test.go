package router

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/auth"
	"github.com/cosminrentea/gobbler/server/kvstore"
	"github.com/cosminrentea/gobbler/server/store"
	"github.com/cosminrentea/gobbler/testutil"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

var (
	dummyPath          = protocol.Path("/dummy")
	dummyMessageWithID = &protocol.Message{ID: 1, Path: dummyPath, Body: []byte("dummy body")}
	dummyMessageBytes  = `/dummy,MESSAGE_ID,user01,phone01,{},1420110000,1
{"Content-Type": "text/plain", "Correlation-Id": "7sdks723ksgqn"}
Hello World`
	chanSize  = 10
	queueSize = 5
)

// Send messages in a zero queued route and expect the route to be closed
// Same test exists for the router
// see router_test.go:TestRoute_IsRemovedIfChannelIsFull
func TestRouteDeliver_sendDirect(t *testing.T) {
	a := assert.New(t)
	r := testRoute()

	for i := 0; i < chanSize; i++ {
		err := r.Deliver(dummyMessageWithID, false)
		a.NoError(err)
	}

	done := make(chan bool)
	go func() {
		r.Deliver(dummyMessageWithID, false)
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Millisecond):
		a.Fail("Message not getting sent!")
	}

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

	a.True(r.invalid)
	a.False(r.consuming)
	a.Equal(0, r.queue.size())
}

func TestRouteDeliver_Invalid(t *testing.T) {
	a := assert.New(t)
	r := testRoute()
	r.invalid = true

	err := r.Deliver(dummyMessageWithID, true)
	a.Equal(ErrInvalidRoute, err)
}

func TestRouteDeliver_QueueSize(t *testing.T) {
	a := assert.New(t)
	// create a route with a queue size
	r := testRoute()
	r.QueueSize = queueSize

	// fill the channel buffer and the queue
	for i := 0; i < chanSize+queueSize; i++ {
		r.Deliver(dummyMessageWithID, true)
	}

	// and the route should close itself if the queue is overflowed
	done := make(chan bool)
	go func() {
		err := r.Deliver(dummyMessageWithID, true)
		a.NotNil(err)
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(40 * time.Millisecond):
		a.Fail("Message not delivering.")
	}
	time.Sleep(10 * time.Millisecond)
	a.True(r.isInvalid())
	a.False(r.isConsuming())
}

func TestRouteDeliver_WithTimeout(t *testing.T) {
	a := assert.New(t)

	// create a route with timeout and infinite queue size
	r := testRoute()
	r.QueueSize = -1 // infinite queue size
	r.Timeout = 10 * time.Millisecond

	// fill the channel buffer
	for i := 0; i < chanSize; i++ {
		r.Deliver(dummyMessageWithID, true)
	}

	// delivering one more message should result in a closed route
	done := make(chan bool)
	go func() {
		err := r.Deliver(dummyMessageWithID, true)
		a.NoError(err)
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(40 * time.Millisecond):
		a.Fail("Message not delivering.")
	}

	time.Sleep(30 * time.Millisecond)
	err := r.Deliver(dummyMessageWithID, true)
	a.Equal(ErrInvalidRoute, err)
	a.True(r.invalid)
	a.False(r.consuming)
}

func TestRoute_CloseTwice(t *testing.T) {
	a := assert.New(t)

	r := testRoute()
	err := r.Close()
	a.Equal(ErrInvalidRoute, err)

	err = r.Close()
	a.Equal(ErrInvalidRoute, err)
}

func TestQueue_ShiftEmpty(t *testing.T) {
	q := newQueue(5)
	q.remove()
	assert.Equal(t, 0, q.size())
}

func testRoute() *Route {
	options := RouteConfig{
		RouteParams: RouteParams{
			"application_id": "appID",
			"user_id":        "userID",
		},
		Path:        protocol.Path(dummyPath),
		ChannelSize: chanSize,
	}
	return NewRoute(options)
}

func TestRoute_messageFilter(t *testing.T) {
	a := assert.New(t)

	route := NewRoute(RouteConfig{
		Path:        "/topic",
		ChannelSize: 1,
		RouteParams: RouteParams{
			"field1": "value1",
			"field2": "value2",
		},
	})

	msg := &protocol.Message{
		ID:   1,
		Path: "/topic",
	}
	route.Deliver(msg, false)

	// test message is received on the channel
	a.True(isMessageReceived(route, msg))

	msg = &protocol.Message{
		ID:   1,
		Path: "/topic",
	}
	msg.SetFilter("field1", "value1")
	route.Deliver(msg, true)
	a.True(isMessageReceived(route, msg))

	msg = &protocol.Message{
		ID:   1,
		Path: "/topic",
	}
	msg.SetFilter("field1", "value1")
	msg.SetFilter("field2", "value2")
	route.Deliver(msg, true)
	a.True(isMessageReceived(route, msg))

	msg = &protocol.Message{
		ID:   1,
		Path: "/topic",
	}
	msg.SetFilter("field1", "value1")
	msg.SetFilter("field2", "value2")
	msg.SetFilter("field3", "value3")
	route.Deliver(msg, true)
	a.False(isMessageReceived(route, msg))

	msg = &protocol.Message{
		ID:   1,
		Path: "/topic",
	}
	msg.SetFilter("field3", "value3")
	route.Deliver(msg, true)
	a.False(isMessageReceived(route, msg))
}

func isMessageReceived(route *Route, msg *protocol.Message) bool {
	select {
	case m, opened := <-route.MessagesChannel():
		if !opened {
			return false
		}

		return m == msg
	case <-time.After(20 * time.Millisecond):
	}

	return false
}

func TestRoute_Provide_ErrMissingFetchRequest(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	routerMock := NewMockRouter(ctrl)
	route := NewRoute(RouteConfig{
		Path: "/fetch_request",
	})
	err := route.Provide(routerMock, false)
	a.Error(err)
	a.Equal(ErrMissingFetchRequest, err)
}

func TestRoute_Provide_Fetch(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	msMock := NewMockMessageStore(ctrl)
	routerMock := NewMockRouter(ctrl)

	routerMock.EXPECT().MessageStore().Return(msMock, nil)

	route := NewRoute(RouteConfig{
		Path:         protocol.Path("/fetch_request"),
		ChannelSize:  5,
		FetchRequest: store.NewFetchRequest("", 0, 0, store.DirectionForward, -1),
	})

	msMock.EXPECT().MaxMessageID("fetch_request").Return(uint64(2), nil).Times(2)

	routerMock.EXPECT().Done().Return(make(chan bool)).AnyTimes()
	routerMock.EXPECT().Fetch(gomock.Any()).Do(func(req *store.FetchRequest) {
		a.Equal(req.Partition, "fetch_request")
		a.Equal(uint64(0), req.StartID)
		a.Equal(uint64(0), req.EndID)
		a.Equal(store.DirectionForward, req.Direction)
		go func() {
			req.StartC <- 2

			// send to messages
			req.Push(1, []byte(strings.Replace(dummyMessageBytes, "MESSAGE_ID", strconv.Itoa(1), 1)))
			req.Push(2, []byte(strings.Replace(dummyMessageBytes, "MESSAGE_ID", strconv.Itoa(2), 1)))
			req.Done()
		}()
	})

	done := make(chan struct{})
	go func() {
		receivedMessages := 0
		for i := 1; i <= 2; i++ {
			select {
			case m, opened := <-route.MessagesChannel():
				if opened {
					receivedMessages++
					a.Equal(uint64(i), m.ID)
				}
			case <-time.After(50 * time.Millisecond):
				a.Fail("Message not received")
			}
		}
		a.Equal(2, receivedMessages)
		close(done)
	}()

	err := route.Provide(routerMock, false)
	a.NoError(err)
	<-done
}

func TestRoute_Provide_WithSubscribe(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	msMock := NewMockMessageStore(ctrl)
	routerMock := NewMockRouter(ctrl)

	routerMock.EXPECT().MessageStore().Return(msMock, nil)

	route := NewRoute(RouteConfig{
		Path:         protocol.Path("/fetch_request"),
		ChannelSize:  4,
		FetchRequest: store.NewFetchRequest("", 0, 0, store.DirectionForward, -1),
	})

	routerMock.EXPECT().Done().Return(make(chan bool)).AnyTimes()
	routerMock.EXPECT().Fetch(gomock.Any()).Do(func(req *store.FetchRequest) {
		a.Equal(req.Partition, "fetch_request")
		a.Equal(uint64(0), req.StartID)
		a.Equal(uint64(0), req.EndID)
		a.Equal(store.DirectionForward, req.Direction)

		go func() {
			req.StartC <- 2

			// send to messages
			req.Push(1, []byte(strings.Replace(dummyMessageBytes, "MESSAGE_ID", strconv.Itoa(1), 1)))
			req.Push(2, []byte(strings.Replace(dummyMessageBytes, "MESSAGE_ID", strconv.Itoa(2), 1)))
			req.Done()
		}()
	})

	msMock.EXPECT().MaxMessageID(gomock.Eq("fetch_request")).Return(uint64(2), nil).Times(2)

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(r *Route) (*Route, error) {
		a.Equal(route, r)

		for i := 3; i <= 4; i++ {
			r.Deliver(&protocol.Message{
				ID:   uint64(i),
				Path: "/fetch_request",
				Body: []byte("dummy"),
			}, true)
		}

		return r, nil
	})

	done := make(chan struct{})
	go func() {
		receivedMessages := 0
		for i := 1; i <= 4; i++ {
			select {
			case m, opened := <-route.MessagesChannel():
				if opened {
					receivedMessages++
					a.Equal(uint64(i), m.ID)
				}
			case <-time.After(50 * time.Millisecond):
				a.Fail("Message not received")
			}
		}
		a.Equal(4, receivedMessages)
		close(done)
	}()

	err := route.Provide(routerMock, true)
	a.NoError(err)
	<-done
}

type startable interface {
	Start() error
}

type stopable interface {
	Stop() error
}

// Test that the route will fetch in case new messages arrived that match the
// fetch request
func TestRoute_Provide_MultipleFetch(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	memoryKV := kvstore.NewMemoryKVStore()

	msMock := NewMockMessageStore(ctrl)
	router := New(auth.AllowAllAccessManager(true), msMock, memoryKV, nil)

	if startable, ok := router.(startable); ok {
		startable.Start()
		if stopable, ok := router.(stopable); ok {
			defer stopable.Stop()
		}
	}

	path := protocol.Path("/fetch_request")

	route := NewRoute(RouteConfig{
		Path:         path,
		ChannelSize:  4,
		FetchRequest: store.NewFetchRequest("", 0, 0, store.DirectionForward, -1),
	})

	block := make(chan struct{})
	maxIDExpect := msMock.EXPECT().MaxMessageID(gomock.Any()).
		Return(uint64(2), nil)
	msMock.EXPECT().Fetch(gomock.Any()).Do(func(req *store.FetchRequest) {
		a.Equal("fetch_request", req.Partition)

		// block the fetch request until pushing some new messages in the router
		go func() {
			<-block
			req.StartC <- 2
			req.Push(1, []byte(strings.Replace(dummyMessageBytes, "MESSAGE_ID", strconv.Itoa(1), 1)))
			req.Push(2, []byte(strings.Replace(dummyMessageBytes, "MESSAGE_ID", strconv.Itoa(2), 1)))
			req.Done()
		}()
	}).After(maxIDExpect)

	msMock.EXPECT().MaxMessageID(gomock.Any()).
		Return(uint64(4), nil).Times(2)
	msMock.EXPECT().Fetch(gomock.Any()).Do(func(req *store.FetchRequest) {
		a.Equal("fetch_request", req.Partition)
		go func() {
			req.StartC <- 2

			// block the fetch request until pushing some new messages in the router
			req.Push(3, []byte(strings.Replace(dummyMessageBytes, "MESSAGE_ID", strconv.Itoa(3), 1)))
			req.Push(4, []byte(strings.Replace(dummyMessageBytes, "MESSAGE_ID", strconv.Itoa(4), 1)))
			req.Done()
		}()
	})

	msMock.EXPECT().StoreMessage(gomock.Any(), gomock.Any()).AnyTimes()

	router.HandleMessage(&protocol.Message{ID: 3, Path: path, Body: []byte("dummy body")})
	router.HandleMessage(&protocol.Message{ID: 4, Path: path, Body: []byte("dummy body")})

	done := make(chan struct{})
	go func() {
		receivedMessages := 0
		for i := 1; i <= 4; i++ {
			select {
			case m, opened := <-route.MessagesChannel():
				if opened {
					receivedMessages++
					a.Equal(uint64(i), m.ID)
				}
			case <-time.After(50 * time.Millisecond):
				a.Fail(fmt.Sprintf("Message not received: %d", i))
			}
		}
		a.Equal(4, receivedMessages)
		close(done)
	}()
	close(block)

	err := route.Provide(router, true)
	a.NoError(err)
	<-done
}

func TestRoute_Provide_EndIDSubscribe(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	msMock := NewMockMessageStore(ctrl)
	routerMock := NewMockRouter(ctrl)

	routerMock.EXPECT().MessageStore().Return(msMock, nil)

	route := NewRoute(RouteConfig{
		Path:         protocol.Path("/fetch_request"),
		ChannelSize:  5,
		FetchRequest: store.NewFetchRequest("", 8, 10, store.DirectionForward, -1),
	})

	msMock.EXPECT().MaxMessageID("fetch_request").Return(uint64(12), nil).Times(2)

	routerMock.EXPECT().Done().Return(make(chan bool)).AnyTimes()
	routerMock.EXPECT().Fetch(gomock.Any()).Do(func(req *store.FetchRequest) {
		a.Equal(req.Partition, "fetch_request")
		a.Equal(uint64(8), req.StartID)
		a.Equal(uint64(10), req.EndID)
		a.Equal(store.DirectionForward, req.Direction)

		go func() {
			req.StartC <- 3

			// send the messages
			req.Push(8, []byte(strings.Replace(dummyMessageBytes, "MESSAGE_ID", strconv.Itoa(8), 1)))
			req.Push(9, []byte(strings.Replace(dummyMessageBytes, "MESSAGE_ID", strconv.Itoa(9), 1)))
			req.Push(10, []byte(strings.Replace(dummyMessageBytes, "MESSAGE_ID", strconv.Itoa(10), 1)))
			req.Done()
		}()
	})

	routerMock.EXPECT().Subscribe(gomock.Eq(route)).Return(route, nil)

	done := make(chan struct{})
	go func() {
		receivedMessages := 0
		for i := 8; i <= 10; i++ {
			select {
			case m, opened := <-route.MessagesChannel():
				if opened {
					receivedMessages++
					a.Equal(uint64(i), m.ID)
				}
			case <-time.After(50 * time.Millisecond):
				a.Fail("Message not received")
			}
		}
		a.Equal(3, receivedMessages)
		close(done)
	}()

	err := route.Provide(routerMock, true)
	a.NoError(err)
	<-done
}
