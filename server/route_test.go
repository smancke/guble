package server

import (
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var (
	dummyPath          = protocol.Path("/dummy")
	dummyMessageWithID = &protocol.Message{ID: 1, Path: dummyPath, Body: []byte("dummy body")}
	chanSize           = 10
	queueSize          = 5
)

// Send messages in a zero queued route and expect the route to be closed
// Same test exists for the router
// see router_test.go:TestRoute_IsRemovedIfChannelIsFull
func TestRouteDeliver_sendDirect(t *testing.T) {
	a := assert.New(t)
	r := testRoute()

	for i := 0; i < chanSize; i++ {
		err := r.Deliver(dummyMessageWithID)
		a.NoError(err)
	}

	done := make(chan bool)
	go func() {
		r.Deliver(dummyMessageWithID)
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Millisecond):
		a.Fail("Message not getting sent!")
	}

	for i := 0; i < chanSize; i++ {
		select {
		case _, open := <-r.MessagesC():
			a.True(open)
		case <-time.After(time.Millisecond * 10):
			a.Fail("error not enough messages in channel")
		}
	}

	// and the channel is closed
	select {
	case _, open := <-r.MessagesC():
		a.False(open)
	default:
		protocol.Debug("len(r.C): %v", len(r.MessagesC()))
		a.Fail("channel was not closed")
	}

	a.True(r.invalid)
	a.False(r.consuming)
	a.Equal(0, r.queue.len())
}

func TestRouteDeliver_Invalid(t *testing.T) {
	a := assert.New(t)
	r := testRoute()
	r.invalid = true

	err := r.Deliver(dummyMessageWithID)
	a.Equal(ErrInvalidRoute, err)
}

func TestRouteDeliver_QueueSize(t *testing.T) {
	defer testutil.EnableDebugForMethod()()

	a := assert.New(t)
	// create a route with a queue size
	r := testRoute().SetQueueSize(queueSize)

	// fill the channel buffer and the queue
	for i := 0; i < chanSize+queueSize; i++ {
		r.Deliver(dummyMessageWithID)
	}

	// and the route should close itself if the queue is overflowed
	done := make(chan bool)
	go func() {
		err := r.Deliver(dummyMessageWithID)
		a.NotNil(err)
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(40 * time.Millisecond):
		a.Fail("Message not delivering.")
	}

	time.Sleep(50 * time.Millisecond)
}

func testRoute() *Route {
	return NewRoute(string(dummyPath), "appID", "userID", make(chan *MessageForRoute, chanSize))
}
