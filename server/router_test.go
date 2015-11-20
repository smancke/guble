package server

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var aTestMessage = []byte("Hello World!")
var chanSize = 1

func TestAddAndRemoveRoutes(t *testing.T) {
	a := assert.New(t)

	// Given a Multiplexer
	router := NewPubSubRouter().Go()

	// when i add two routes in the same path
	channel := make(chan []byte)
	routeBlah1 := router.Subscribe(NewRoute("/blah", channel))
	routeBlah2 := router.Subscribe(NewRoute("/blah", channel))

	// and one route in another path
	routeFoo := router.Subscribe(NewRoute("/foo", channel))

	fmt.Printf("%+v\n", router)

	// then
	// they have correct ids
	a.Equal(16, len(routeBlah1.Id))
	a.NotEqual(routeBlah1.Id, routeBlah2.Id)
	a.NotEqual(routeBlah2.Id, routeFoo.Id)

	// and the routes are stored
	a.Equal(2, len(router.routes[Path("/blah")]))
	a.Equal(router.routes[Path("/blah")][0].Id, routeBlah1.Id)
	a.Equal(router.routes[Path("/blah")][1].Id, routeBlah2.Id)

	a.Equal(1, len(router.routes[Path("/foo")]))
	a.Equal(router.routes[Path("/foo")][0].Id, routeFoo.Id)

	// WHEN i remove routes
	router.Unsubscribe(routeBlah1)
	router.Unsubscribe(routeFoo)

	// then they are gone
	a.Equal(1, len(router.routes[Path("/blah")]))
	a.Equal(router.routes[Path("/blah")][0].Id, routeBlah2.Id)

	a.Nil(router.routes[Path("/foo")])
}

func TestSimpleMessageSending(t *testing.T) {
	a := assert.New(t)

	// Given a Multiplexer with route
	router, r := aRouterRoute()

	// when i send a message to the route
	router.HandleMessage(Message{Path: r.Path, Body: aTestMessage})

	// then I can receive it a short time later
	assertChannelContainsMessage(a, r.C, aTestMessage)
}

func TestRoutingWithSubTopics(t *testing.T) {
	a := assert.New(t)

	// Given a Multiplexer with route
	router := NewPubSubRouter().Go()
	channel := make(chan []byte)
	r := router.Subscribe(NewRoute("/blah", channel))

	// when i send a message to a subroute
	router.HandleMessage(Message{Path: "/blah/blub", Body: aTestMessage})

	// then I can receive the message
	assertChannelContainsMessage(a, r.C, aTestMessage)

	// but, when i send a message to a resource, which is just a substring
	router.HandleMessage(Message{Path: "/blahblub", Body: aTestMessage})

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
	router, r := aRouterRoute()
	// where the channel is full of messages
	for i := 0; i < chanSize; i++ {
		router.HandleMessage(Message{Path: r.Path, Body: aTestMessage})
	}

	// when I send one more message
	done := make(chan []byte, 1)
	go func() {
		router.HandleMessage(Message{Path: r.Path, Body: aTestMessage})
		done <- []byte("done")
	}()

	// then I the sending reurns immediately
	assertChannelContainsMessage(a, done, []byte("done"))

	// and thwo messaes are recieveable
	for i := 0; i < chanSize; i++ {
		assertChannelContainsMessage(a, r.C, aTestMessage)
	}

	assertChannelContainsMessage(a, r.C, aTestMessage)
}

func aRouterRoute() (*PubSubRouter, *Route) {

	router := NewPubSubRouter().Go()
	return router,
		router.Subscribe(NewRoute("/blah", make(chan []byte)))
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
