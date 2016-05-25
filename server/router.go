package server

import (
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/auth"
	"github.com/smancke/guble/store"
	"runtime"
	"strings"
	"time"
)

// Router interface provides mechanism for PubSub messaging
type Router interface {
	Module
	KVStore() (store.KVStore, error)
	AccessManager() (auth.AccessManager, error)
	MessageStore() (store.MessageStore, error)

	Subscribe(r *Route) (*Route, error)
	Unsubscribe(r *Route)
	HandleMessage(message *protocol.Message) error
}

// Helper struct to pass `Route` to subscription channel and provide a
// notification channel
type subRequest struct {
	route      *Route
	doneNotify chan bool
}

type router struct {
	// mapping the path to the route slice
	routes          map[protocol.Path][]Route
	messageIn       chan *protocol.Message
	subscribeChan   chan subRequest
	unsubscribeChan chan subRequest
	stop            chan bool

	// external services
	accessManager auth.AccessManager
	messageStore  store.MessageStore
	kvStore       store.KVStore
}

// NewRouter returns a pointer to Router
func NewRouter(
	accessManager auth.AccessManager,
	messageStore store.MessageStore,
	kvStore store.KVStore) Router {
	return &router{
		routes:          make(map[protocol.Path][]Route),
		messageIn:       make(chan *protocol.Message, 500),
		subscribeChan:   make(chan subRequest, 10),
		unsubscribeChan: make(chan subRequest, 10),
		stop:            make(chan bool, 1),

		accessManager: accessManager,
		messageStore:  messageStore,
		kvStore:       kvStore,
	}
}

func (router *router) Start() error {
	if router.accessManager == nil {
		panic("AccessManager not set. Cannot start.")
	}

	go func() {
		for {
			func() {
				defer protocol.PanicLogger()

				select {
				case message := <-router.messageIn:
					router.handleMessage(message)
					runtime.Gosched()
				case subscriber := <-router.subscribeChan:
					router.subscribe(subscriber.route)
					subscriber.doneNotify <- true
				case unsubscriber := <-router.unsubscribeChan:
					router.unsubscribe(unsubscriber.route)
					unsubscriber.doneNotify <- true
				case <-router.stop:
					router.closeAllRoutes()
					protocol.Debug("stopping message router")
					break
				}
			}()
		}
	}()

	return nil
}

func (router *router) Check() error {
	return nil
}

// Stop stops the router by closing the stop channel
func (router *router) Stop() error {
	close(router.stop)
	return nil
}

// Subscribe adds a route to the subscribers.
// If there is already a route with same Application Id and Path, it will be replaced.
func (router *router) Subscribe(r *Route) (*Route, error) {
	protocol.Debug("subscribe %v, %v, %v", router.accessManager, r.UserID, r.Path)
	accessAllowed := router.accessManager.IsAllowed(auth.READ, r.UserID, r.Path)
	if !accessAllowed {
		return r, &PermissionDeniedError{r.UserID, auth.READ, r.Path}
	}
	req := subRequest{
		route:      r,
		doneNotify: make(chan bool),
	}
	router.subscribeChan <- req
	<-req.doneNotify
	return r, nil
}

func (router *router) subscribe(r *Route) {
	protocol.Info("subscribe applicationID=%v, path=%v", r.ApplicationID, r.Path)

	routeList, present := router.routes[r.Path]
	if !present {
		routeList = []Route{}
		router.routes[r.Path] = routeList
	}

	// try to remove, to avoid double subscriptions of the same app
	routeList = remove(routeList, r)

	router.routes[r.Path] = append(routeList, *r)
}

func (router *router) Unsubscribe(r *Route) {
	req := subRequest{
		route:      r,
		doneNotify: make(chan bool),
	}
	router.unsubscribeChan <- req
	<-req.doneNotify
}

func (router *router) unsubscribe(r *Route) {
	protocol.Info("unsubscribe applicationID=%v, path=%v", r.ApplicationID, r.Path)
	routeList, present := router.routes[r.Path]
	if !present {
		return
	}
	router.routes[r.Path] = remove(routeList, r)
	if len(router.routes[r.Path]) == 0 {
		delete(router.routes, r.Path)
	}
}

func (router *router) HandleMessage(message *protocol.Message) error {
	protocol.Debug("Route.HandleMessage: %v %v", message.UserID, message.Path)
	if !router.accessManager.IsAllowed(auth.WRITE, message.UserID, message.Path) {
		return &PermissionDeniedError{message.UserID, auth.WRITE, message.Path}
	}

	return router.storeMessage(message)
}

// Assign the new message id and store it and handle by passing the stored message
// to the messageIn channel
func (router *router) storeMessage(msg *protocol.Message) error {
	txCallback := func(msgId uint64) []byte {
		msg.ID = msgId
		msg.Time = time.Now().Unix()
		return msg.Bytes()
	}

	if err := router.messageStore.StoreTx(msg.Path.Partition(), txCallback); err != nil {
		protocol.Err("error storing message in partition %v: %v", msg.Path.Partition(), err)
		return err
	}

	if float32(len(router.messageIn))/float32(cap(router.messageIn)) > 0.9 {
		protocol.Warn("router.messageIn channel very full: current=%v, max=%v\n", len(router.messageIn), cap(router.messageIn))
		time.Sleep(time.Millisecond)
	}

	router.messageIn <- msg
	return nil
}

func (router *router) handleMessage(message *protocol.Message) {
	if protocol.InfoEnabled() {
		protocol.Info("routing message: %v", message.Metadata())
	}

	for currentRoutePath, currentRouteList := range router.routes {
		if matchesTopic(message.Path, currentRoutePath) {
			for _, route := range currentRouteList {
				router.deliverMessage(route, message)
			}
		}
	}
}

func (router *router) deliverMessage(route Route, message *protocol.Message) {
	defer protocol.PanicLogger()
	select {
	case route.C <- MsgAndRoute{Message: message, Route: &route}:
	// fine, we could send the message
	default:
		protocol.Info("queue was full, closing delivery for route=%v to applicationID=%v", route.Path, route.ApplicationID)
		close(route.C)
		router.unsubscribe(&route)
	}
}

func (router *router) closeAllRoutes() {
	for _, currentRouteList := range router.routes {
		for _, route := range currentRouteList {
			close(route.C)
			router.unsubscribe(&route)
		}
	}
}

func copyOf(message []byte) []byte {
	messageCopy := make([]byte, len(message))
	copy(messageCopy, message)
	return messageCopy
}

// Test wether the supplied routePath matches the message topic
func matchesTopic(messagePath, routePath protocol.Path) bool {
	messagePathLen := len(string(messagePath))
	routePathLen := len(string(routePath))
	return strings.HasPrefix(string(messagePath), string(routePath)) &&
		(messagePathLen == routePathLen ||
			(messagePathLen > routePathLen && string(messagePath)[routePathLen] == '/'))
}

// remove a route from the supplied list,
// based on same ApplicationID id and same path
func remove(slice []Route, route *Route) []Route {
	position := -1
	for p, r := range slice {
		if r.ApplicationID == route.ApplicationID && r.Path == route.Path {
			position = p
		}
	}
	if position == -1 {
		return slice
	}
	return append(slice[:position], slice[position+1:]...)
}

// AccessManager returns the `accessManager` provided for the router
func (router *router) AccessManager() (auth.AccessManager, error) {
	if router.accessManager == nil {
		return nil, ErrServiceNotProvided
	}
	return router.accessManager, nil
}

// MessageStore returns the `messageStore` provided for the router
func (router *router) MessageStore() (store.MessageStore, error) {
	if router.messageStore == nil {
		return nil, ErrServiceNotProvided
	}
	return router.messageStore, nil
}

// KVStore returns the `kvStore` provided for the router
func (router *router) KVStore() (store.KVStore, error) {
	if router.kvStore == nil {
		return nil, ErrServiceNotProvided
	}
	return router.kvStore, nil
}
