package server

import (
	"github.com/rs/xid"
	"log"
	"runtime"
	"strings"
	"time"
)

type Path string

type Route struct {
	Id   string
	Path Path
	C    chan []byte
}

type SubRequest struct {
	route      Route
	doneNotify chan bool
}

type Message struct {
	id   int64
	path Path
	body []byte
}

type MsgMultiplexer struct {
	// mapping the path to the route slice
	routes           map[Path][]Route
	messageIn        chan Message
	subscribe        chan SubRequest
	unsubscribe      chan SubRequest
	stop             chan bool
	routeChannelSize int
}

func NewMultiplexer() *MsgMultiplexer {
	return &MsgMultiplexer{
		routes:           make(map[Path][]Route),
		messageIn:        make(chan Message, 500),
		subscribe:        make(chan SubRequest, 10),
		unsubscribe:      make(chan SubRequest, 10),
		stop:             make(chan bool, 1),
		routeChannelSize: 1,
	}
}

func (mux *MsgMultiplexer) Go() *MsgMultiplexer {
	go func() {
		//log.Println("DEBUG: starting message multiplexer")
		for {
			//log.Println("DEBUG: GO: next select")

			select {
			case message := <-mux.messageIn:
				//log.Println("DEBUG: GO: before handleMessage")
				mux.handleMessage(message)
				//log.Println("DEBUG: GO: after handleMessage")
			case subscriber := <-mux.subscribe:
				mux.addRoute(subscriber.route)
				subscriber.doneNotify <- true
			case unsubscriber := <-mux.unsubscribe:
				mux.removeRoute(unsubscriber.route)
				unsubscriber.doneNotify <- true
			case <-mux.stop:
				mux.closeAllRoutes()
				//log.Println("DEBUG: stopping message multiplexer")
				break
			}
			runtime.Gosched()
		}
	}()
	return mux
}

func (mux *MsgMultiplexer) Stop() {
	mux.stop <- true
	runtime.Gosched()
}

func (mux *MsgMultiplexer) AddRoute(r Route) Route {
	req := SubRequest{
		route:      r,
		doneNotify: make(chan bool),
	}
	mux.subscribe <- req
	<-req.doneNotify
	return r
}

func (mux *MsgMultiplexer) addRoute(r Route) Route {
	routeList, present := mux.routes[r.Path]
	if !present {
		routeList = []Route{}
		mux.routes[r.Path] = routeList
	}
	mux.routes[r.Path] = append(routeList, r)
	return r
}

func (mux *MsgMultiplexer) AddNewRoute(path string) Route {
	return mux.AddRoute(
		Route{
			Id:   xid.New().String(),
			Path: Path(path),
			C:    make(chan []byte, mux.routeChannelSize),
		})
}

func (mux *MsgMultiplexer) RemoveRoute(r Route) {
	req := SubRequest{
		route:      r,
		doneNotify: make(chan bool),
	}
	mux.unsubscribe <- req
	<-req.doneNotify
}

func (mux *MsgMultiplexer) removeRoute(r Route) {
	routeList, present := mux.routes[r.Path]
	if !present {
		return
	}
	mux.routes[r.Path] = remove(routeList, r)
	if len(mux.routes[r.Path]) == 0 {
		delete(mux.routes, r.Path)
	}
}

func (mux *MsgMultiplexer) HandleMessage(message Message) {
	mux.messageIn <- message
	runtime.Gosched()
}

func (mux *MsgMultiplexer) handleMessage(message Message) {
	log.Printf("INFO: handle message=%v, len=%v", string(message.body), len(message.body))

	log.Printf("DEBUG: number of routes =%v", len(mux.routes))

	for currentRoutePath, currentRouteList := range mux.routes {
		if matchesTopic(message.path, currentRoutePath) {
			mux.deliverMessage(copyOf(message.body), currentRouteList)
		}
	}
}

func (mux *MsgMultiplexer) deliverMessage(message []byte, deliverRouteList []Route) {
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
					mux.removeRoute(route)
					close(route.C)
				}
			}(route)
		}
		//log.Println("DEBUG: GO: deliverMessage->done delivery select")

	}
}

func (mux *MsgMultiplexer) closeAllRoutes() {
	for _, currentRouteList := range mux.routes {
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
func matchesTopic(messagePath, routePath Path) bool {
	messagePathLen := len(string(messagePath))
	routePathLen := len(string(routePath))
	return strings.HasPrefix(string(messagePath), string(routePath)) &&
		(messagePathLen == routePathLen ||
			(messagePathLen > routePathLen && string(messagePath)[routePathLen] == '/'))
}

func remove(slice []Route, route Route) []Route {
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
