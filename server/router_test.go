package server

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	guble "github.com/smancke/guble/guble"
)

var aTestByteMessage = []byte("Hello World!")
var chanSize = 10

func TestAddAndRemoveRoutes(t *testing.T) {
	a := assert.New(t)

	// Given a Multiplexer
	router := NewPubSubRouter().Go()

	// when i add two routes in the same path
	channel := make(chan *guble.Message, chanSize)
	closeRouteByRouter := make(chan string)
	routeBlah1 := router.Subscribe(NewRoute("/blah", channel, closeRouteByRouter, "appid01"))
	routeBlah2 := router.Subscribe(NewRoute("/blah", channel, closeRouteByRouter, "appid01"))

	// and one route in another path
	routeFoo := router.Subscribe(NewRoute("/foo", channel, closeRouteByRouter, "appid01"))

	fmt.Printf("%+v\n", router)

	// then
	// they have correct ids
	a.Equal(16, len(routeBlah1.Id))
	a.NotEqual(routeBlah1.Id, routeBlah2.Id)
	a.NotEqual(routeBlah2.Id, routeFoo.Id)

	// and the routes are stored
	a.Equal(2, len(router.routes[guble.Path("/blah")]))
	a.Equal(router.routes[guble.Path("/blah")][0].Id, routeBlah1.Id)
	a.Equal(router.routes[guble.Path("/blah")][1].Id, routeBlah2.Id)

	a.Equal(1, len(router.routes[guble.Path("/foo")]))
	a.Equal(router.routes[guble.Path("/foo")][0].Id, routeFoo.Id)

	// WHEN i remove routes
	router.Unsubscribe(routeBlah1)
	router.Unsubscribe(routeFoo)

	// then they are gone
	a.Equal(1, len(router.routes[guble.Path("/blah")]))
	a.Equal(router.routes[guble.Path("/blah")][0].Id, routeBlah2.Id)

	a.Nil(router.routes[guble.Path("/foo")])
}

func TestSimpleMessageSending(t *testing.T) {
	a := assert.New(t)

	// Given a Multiplexer with route
	router, r := aRouterRoute()

	// when i send a message to the route
	router.HandleMessage(&guble.Message{Path: r.Path, Body: aTestByteMessage})

	// then I can receive it a short time later
	assertChannelContainsMessage(a, r.C, aTestByteMessage)
}

func TestRoutingWithSubTopics(t *testing.T) {
	a := assert.New(t)

	// Given a Multiplexer with route
	router := NewPubSubRouter().Go()
	channel := make(chan *guble.Message, chanSize)
	closeRouteByRouter := make(chan string)
	r := router.Subscribe(NewRoute("/blah", channel, closeRouteByRouter, "appid01"))

	// when i send a message to a subroute
	router.HandleMessage(&guble.Message{Path: "/blah/blub", Body: aTestByteMessage})

	// then I can receive the message
	assertChannelContainsMessage(a, r.C, aTestByteMessage)

	// but, when i send a message to a resource, which is just a substring
	router.HandleMessage(&guble.Message{Path: "/blahblub", Body: aTestByteMessage})

	// then the message gets not delivered
	a.Equal(0, len(r.C))
}

func TestMatchesTopic(t *testing.T) {
	for _, test := range []struct {
		messagePath guble.Path
		routePath   guble.Path
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
	a := assert.New(t)

	// Given a Multiplexer with route
	router, r := aRouterRoute()
	// where the channel is full of messages
	for i := 0; i < chanSize; i++ {
		router.HandleMessage(&guble.Message{Path: r.Path, Body: aTestByteMessage})
	}

	// when I send one more message
	done := make(chan bool, 1)
	go func() {
		router.HandleMessage(&guble.Message{Path: r.Path, Body: aTestByteMessage})
		done <- true
	}()

	// then: the it returns immediately
	select {
	case <-done:
	case <-time.After(time.Millisecond * 10):
		a.Fail("Not returning!")
	}

	// and the close channel contains this route
	select {
	case routeId := <-r.CloseRouteByRouter:
		a.Equal(r.Id, routeId)
	case <-time.After(time.Millisecond):
		a.Fail("no close message received")
	}
}

func aRouterRoute() (*PubSubRouter, *Route) {
	router := NewPubSubRouter().Go()
	return router, router.Subscribe(NewRoute("/blah", make(chan *guble.Message, chanSize), make(chan string, 1), "appid01"))
}

func assertChannelContainsMessage(a *assert.Assertions, c chan *guble.Message, msg []byte) {
	//log.Println("DEBUG: start assertChannelContainsMessage-> select")
	select {
	case msgBack := <-c:
		a.Equal(string(msg), string(msgBack.Body))
	case <-time.After(time.Millisecond):
		a.Fail("No message received")
	}
}
