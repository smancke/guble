package server

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var aTestMessage = []byte("Hello World!")

func TestAddAndRemoveRoutes(t *testing.T) {
	a := assert.New(t)

	// Given a Multiplexer
	mux := NewMultiplexer().Go()

	// when i add two routes in the same path
	routeBlah1 := mux.AddNewRoute("/blah")
	routeBlah2 := mux.AddNewRoute("/blah")

	// and one route in another path
	routeFoo := mux.AddNewRoute("/foo")

	fmt.Printf("%+v\n", mux)

	// then
	// they have correct ids
	a.Equal(16, len(routeBlah1.Id))
	a.NotEqual(routeBlah1.Id, routeBlah2.Id)
	a.NotEqual(routeBlah2.Id, routeFoo.Id)

	// and the routes are stored
	a.Equal(2, len(mux.routes[Path("/blah")]))
	a.Equal(mux.routes[Path("/blah")][0].Id, routeBlah1.Id)
	a.Equal(mux.routes[Path("/blah")][1].Id, routeBlah2.Id)

	a.Equal(1, len(mux.routes[Path("/foo")]))
	a.Equal(mux.routes[Path("/foo")][0].Id, routeFoo.Id)

	// WHEN i remove routes
	mux.RemoveRoute(routeBlah1)
	mux.RemoveRoute(routeFoo)

	// then they are gone
	a.Equal(1, len(mux.routes[Path("/blah")]))
	a.Equal(mux.routes[Path("/blah")][0].Id, routeBlah2.Id)

	a.Nil(mux.routes[Path("/foo")])
}

func TestSimpleMessageSending(t *testing.T) {
	a := assert.New(t)

	// Given a Multiplexer with route
	mux, r := aMuxRoute()

	// when i send a message to the route
	mux.HandleMessage(Message{path: r.Path, body: aTestMessage})

	// then I can receive it a short time later
	assertChannelContainsMessage(a, r.C, aTestMessage)
}

func TestRoutingWithSubTopics(t *testing.T) {
	a := assert.New(t)

	// Given a Multiplexer with route
	mux := NewMultiplexer().Go()
	r := mux.AddNewRoute("/blah")

	// when i send a message to a subroute
	mux.HandleMessage(Message{path: "/blah/blub", body: aTestMessage})

	// then I can receive the message
	assertChannelContainsMessage(a, r.C, aTestMessage)

	// but, when i send a message to a resource, which is just a substring
	mux.HandleMessage(Message{path: "/blahblub", body: aTestMessage})

	// then the message gets not delivered
	a.Equal(0, len(r.C))
}

func TestMatchesTopic(t *testing.T) {
	for _, test := range []struct {
		messagePath Path
		routePath   Path
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

func TestCallerIsNotBlockedIfTheChannelIsFull(t *testing.T) {
	a := assert.New(t)

	// Given a Multiplexer with route
	mux, r := aMuxRoute()
	// where the channel is full of messages
	for i := 0; i < mux.routeChannelSize; i++ {
		mux.HandleMessage(Message{path: r.Path, body: aTestMessage})
	}

	// when I send one more message
	done := make(chan []byte, 1)
	go func() {
		mux.HandleMessage(Message{path: r.Path, body: aTestMessage})
		done <- []byte("done")
	}()

	// then I the sending reurns immediately
	assertChannelContainsMessage(a, done, []byte("done"))

	// and thwo messaes are recieveable
	for i := 0; i < mux.routeChannelSize; i++ {
		assertChannelContainsMessage(a, r.C, aTestMessage)
	}

	assertChannelContainsMessage(a, r.C, aTestMessage)
}

func aMuxRoute() (*MsgMultiplexer, Route) {
	mux := NewMultiplexer().Go()
	return mux,
		mux.AddNewRoute("/blah")
}

func assertChannelContainsMessage(a *assert.Assertions, c chan []byte, msg []byte) {
	//log.Println("DEBUG: start assertChannelContainsMessage-> select")
	select {
	case msgBack := <-c:
		a.Equal(msg, msgBack)
	case <-time.After(time.Millisecond):
		a.Fail("No message received")
	}
}
