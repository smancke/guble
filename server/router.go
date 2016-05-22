package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/server/auth"
	"github.com/smancke/guble/store"
	"runtime"
	"strings"
	"time"
)

// Helper struct to pass `Route` to subscription channel and provide a
// notification channel
type subRequest struct {
	route      *Route
	doneNotify chan bool
}

// PubSubRouter is the core that handles messages passing them to subscribers
type PubSubRouter struct {
	// mapping the path to the route slice
	routes          map[guble.Path][]Route
	messageIn       chan *guble.Message
	subscribeChan   chan subRequest
	unsubscribeChan chan subRequest
	stop            chan bool

	// external services
	accessManager auth.AccessManager
	messageStore  store.MessageStore
	kvStore       store.KVStore
}

// NewPubSubRouter returns a pointer to PubSubRouter
func NewPubSubRouter(
	accessManager auth.AccessManager,
	messageStore store.MessageStore,
	kvStore store.KVStore) *PubSubRouter {
	return &PubSubRouter{
		routes:          make(map[guble.Path][]Route),
		messageIn:       make(chan *guble.Message, 500),
		subscribeChan:   make(chan subRequest, 10),
		unsubscribeChan: make(chan subRequest, 10),
		stop:            make(chan bool, 1),

		accessManager: accessManager,
		messageStore:  messageStore,
		kvStore:       kvStore,
	}
}

func (router *PubSubRouter) SetAccessManager(accessManager auth.AccessManager) {
	router.accessManager = accessManager
}

func (router *PubSubRouter) Start() error {
	if router.accessManager == nil {
		panic("AccessManager not set. Cannot start.")
	}

	go func() {
		for {
			func() {
				defer guble.PanicLogger()

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
					guble.Debug("stopping message router")
					break
				}
			}()
		}
	}()

	return nil
}

// Stop stops the router by closing the stop channel
func (router *PubSubRouter) Stop() error {
	close(router.stop)
	return nil
}

// Add a route to the subscribers.
// If there is already a route with same Application Id and Path, it will be replaced.
func (router *PubSubRouter) Subscribe(r *Route) (*Route, error) {
	guble.Debug("subscribe %v, %v, %v", router.accessManager, r.UserID, r.Path)
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

func (router *PubSubRouter) subscribe(r *Route) {
	guble.Info("subscribe applicationID=%v, path=%v", r.ApplicationID, r.Path)

	routeList, present := router.routes[r.Path]
	if !present {
		routeList = []Route{}
		router.routes[r.Path] = routeList
	}

	// try to remove, to avoid double subscriptions of the same app
	routeList = remove(routeList, r)

	router.routes[r.Path] = append(routeList, *r)
}

func (router *PubSubRouter) Unsubscribe(r *Route) {
	req := subRequest{
		route:      r,
		doneNotify: make(chan bool),
	}
	router.unsubscribeChan <- req
	<-req.doneNotify
}

func (router *PubSubRouter) unsubscribe(r *Route) {
	guble.Info("unsubscribe applicationID=%v, path=%v", r.ApplicationID, r.Path)
	routeList, present := router.routes[r.Path]
	if !present {
		return
	}
	router.routes[r.Path] = remove(routeList, r)
	if len(router.routes[r.Path]) == 0 {
		delete(router.routes, r.Path)
	}
}

func (router *PubSubRouter) HandleMessage(message *guble.Message) error {
	guble.Debug("Route.HandleMessage: %v %v", message.PublisherUserId, message.Path)
	if !router.accessManager.IsAllowed(auth.WRITE, message.PublisherUserId, message.Path) {
		return &PermissionDeniedError{message.PublisherUserId, auth.WRITE, message.Path}
	}

	if float32(len(router.messageIn))/float32(cap(router.messageIn)) > 0.9 {
		guble.Warn("router.messageIn channel very full: current=%v, max=%v\n", len(router.messageIn), cap(router.messageIn))
		time.Sleep(time.Millisecond)
	}
	router.messageIn <- message
	return nil
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
	case route.C <- MsgAndRoute{Message: message, Route: &route}:
	// fine, we could send the message
	default:
		guble.Info("queue was full, closing delivery for route=%v to applicationID=%v", route.Path, route.ApplicationID)
		close(route.C)
		router.unsubscribe(&route)
	}
}

func (router *PubSubRouter) closeAllRoutes() {
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
func matchesTopic(messagePath, routePath guble.Path) bool {
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
func (p *PubSubRouter) AccessManager() (auth.AccessManager, error) {
	if p.accessManager == nil {
		return nil, ErrServiceNotProvided
	}
	return p.accessManager, nil
}

// MessageStore returns the `messageStore` provided for the router
func (p *PubSubRouter) MessageStore() (store.MessageStore, error) {
	if p.messageStore == nil {
		return nil, ErrServiceNotProvided
	}
	return p.messageStore, nil
}

// KVStore returns the `kvStore` provided for the router
func (p *PubSubRouter) KVStore() (store.KVStore, error) {
	if p.kvStore == nil {
		return nil, ErrServiceNotProvided
	}
	return p.kvStore, nil
}
