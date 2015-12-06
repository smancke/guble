package server

import (
	"runtime"
	"strings"

	guble "github.com/smancke/guble/guble"
)

type SubscriptionRequest struct {
	route      *Route
	doneNotify chan bool
}

type PubSubRouter struct {
	// mapping the path to the route slice
	routes          map[guble.Path][]Route
	messageIn       chan *guble.Message
	subscribeChan   chan SubscriptionRequest
	unsubscribeChan chan SubscriptionRequest
	stop            chan bool
}

func NewPubSubRouter() *PubSubRouter {
	return &PubSubRouter{
		routes:          make(map[guble.Path][]Route),
		messageIn:       make(chan *guble.Message, 500),
		subscribeChan:   make(chan SubscriptionRequest, 10),
		unsubscribeChan: make(chan SubscriptionRequest, 10),
		stop:            make(chan bool, 1),
	}
}

func (router *PubSubRouter) Go() *PubSubRouter {
	go func() {
		for {
			func() {
				defer guble.PanicLogger()

				select {
				case message := <-router.messageIn:
					//log.Println("DEBUG: GO: before handleMessage")
					router.handleMessage(message)
					//log.Println("DEBUG: GO: after handleMessage")
				case subscriber := <-router.subscribeChan:
					router.subscribe(subscriber.route)
					subscriber.doneNotify <- true
				case unsubscriber := <-router.unsubscribeChan:
					router.unsubscribe(unsubscriber.route)
					unsubscriber.doneNotify <- true
				case <-router.stop:
					router.closeAllRoutes()
					//log.Println("DEBUG: stopping message multiplexer")
					break
				}
				runtime.Gosched()
			}()
		}
	}()
	return router
}

func (router *PubSubRouter) Stop() {
	router.stop <- true
	runtime.Gosched()
}

func (router *PubSubRouter) Subscribe(r *Route) *Route {
	req := SubscriptionRequest{
		route:      r,
		doneNotify: make(chan bool),
	}
	router.subscribeChan <- req
	<-req.doneNotify
	return r
}

func (router *PubSubRouter) subscribe(r *Route) {
	routeList, present := router.routes[r.Path]
	if !present {
		routeList = []Route{}
		router.routes[r.Path] = routeList
	}
	router.routes[r.Path] = append(routeList, *r)
}

func (router *PubSubRouter) Unsubscribe(r *Route) {
	req := SubscriptionRequest{
		route:      r,
		doneNotify: make(chan bool),
	}
	router.unsubscribeChan <- req
	<-req.doneNotify
}

func (router *PubSubRouter) unsubscribe(r *Route) {
	routeList, present := router.routes[r.Path]
	if !present {
		return
	}
	router.routes[r.Path] = remove(routeList, r)
	if len(router.routes[r.Path]) == 0 {
		delete(router.routes, r.Path)
	}
}

func (router *PubSubRouter) HandleMessage(message *guble.Message) {
	router.messageIn <- message
	runtime.Gosched()
}

func (router *PubSubRouter) handleMessage(message *guble.Message) {
	if guble.InfoEnabled() {
		guble.Info("routing message: %v", message.MetadataLine())
	}

	for currentRoutePath, currentRouteList := range router.routes {
		if matchesTopic(message.Path, currentRoutePath) {
			for _, route := range currentRouteList {
				router.deliverMessage(route, message)
			}
		}
	}
}

func (router *PubSubRouter) deliverMessage(route Route, message *guble.Message) {
	defer guble.PanicLogger()
	select {
	case route.C <- message:
		// fine, we could send the message
	default:
		guble.Info("queue was full, closing delivery for route=%v to applicationId=%v", route.Path, route.ApplicationId)
		// the message channel is blocked,
		// so we notify this route, that we stopped delivery and kick it out
		select {
		case route.CloseRouteByRouter <- route.Id:
		default:
			// ignore, if the closedByRouter already was full
		}
		router.unsubscribe(&route)
	}
}

func (router *PubSubRouter) closeAllRoutes() {
	for _, currentRouteList := range router.routes {
		for _, route := range currentRouteList {
			select {
			case route.CloseRouteByRouter <- route.Id:
			default:
				// ignore, if the closedByRouter already was full
			}
		}
	}
}

func copyOf(message []byte) []byte {
	messageCopy := make([]byte, len(message))
	copy(messageCopy, message)
	return messageCopy
}

// Test wether the supplied routePath matches the message topic
func matchesTopic(messagePath, routePath guble.Path) bool {
	messagePathLen := len(string(messagePath))
	routePathLen := len(string(routePath))
	return strings.HasPrefix(string(messagePath), string(routePath)) &&
		(messagePathLen == routePathLen ||
			(messagePathLen > routePathLen && string(messagePath)[routePathLen] == '/'))
}

func remove(slice []Route, route *Route) []Route {
	position := -1
	for p, r := range slice {
		if r.Id == route.Id {
			position = p
		}
	}
	if position == -1 {
		return slice
	}
	return append(slice[:position], slice[position+1:]...)
}
