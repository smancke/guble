package server

import (
	"log"
	"runtime"
	"strings"
	"time"

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
		//log.Println("DEBUG: starting message multiplexer")
		for {
			//log.Println("DEBUG: GO: next select")

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
	log.Printf("INFO: handle message=%v, len=%v", string(message.Body), len(message.Body))

	log.Printf("DEBUG: number of routes =%v", len(router.routes))

	for currentRoutePath, currentRouteList := range router.routes {
		if matchesTopic(message.Path, currentRoutePath) {
			router.deliverMessage(message, currentRouteList)
		}
	}
}

func (router *PubSubRouter) deliverMessage(message *guble.Message, deliverRouteList []Route) {
	for _, route := range deliverRouteList {
		//log.Println("DEBUG: GO: deliverMessage->going into delivery select ..")

		select {
		case route.C <- message:
			//log.Println("DEBUG: GO: deliverMessage->delivered message in channel")
			// fine, we could send the message
		default:
			// the channel is blocked
			//log.Println("DEBUG: GO: deliverMessage-> the channel is blocked, starting go routine with timeout ..")

			// lets send asynchronous with a timeout
			go func(r Route) {
				select {
				case r.C <- message:
					// fine!
				case <-time.After(time.Second * 3):
					log.Printf("WARN: ran into timeout when sending message to route=%v, closing path now", route.Path)
					router.unsubscribe(&route)
					close(route.C)
				}
			}(route)
		}
		//log.Println("DEBUG: GO: deliverMessage->done delivery select")

	}
}

func (router *PubSubRouter) closeAllRoutes() {
	for _, currentRouteList := range router.routes {
		for _, route := range currentRouteList {
			close(route.C)
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
