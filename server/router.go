package server

import (
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/auth"
	"github.com/smancke/guble/store"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Router interface provides mechanism for PubSub messaging
type Router interface {
	KVStore() (store.KVStore, error)
	AccessManager() (auth.AccessManager, error)
	MessageStore() (store.MessageStore, error)

	Subscribe(r *Route) (*Route, error)
	Unsubscribe(r *Route)
	HandleMessage(message *protocol.Message) error
}

// Helper struct to pass `Route` to subscription channel and provide a notification channel.
type subRequest struct {
	route      *Route
	doneNotify chan bool
}

type router struct {
	// mapping the path to the route slice
	routes       map[protocol.Path][]*Route
	handleC      chan *protocol.Message
	subscribeC   chan subRequest
	unsubscribeC chan subRequest

	// Channel that signals stop of the router
	stop chan bool
	// marks that the router is in stopping process
	// no incoming messages are accepted
	stopping bool

	// Add any operation that we need to wait upon here
	wg sync.WaitGroup

	// external 'services'
	accessManager auth.AccessManager
	messageStore  store.MessageStore
	kvStore       store.KVStore
}

// NewRouter returns a pointer to Router
func NewRouter(accessManager auth.AccessManager, messageStore store.MessageStore, kvStore store.KVStore) Router {
	return &router{
		routes:       make(map[protocol.Path][]*Route),
		handleC:      make(chan *protocol.Message, 500),
		subscribeC:   make(chan subRequest, 10),
		unsubscribeC: make(chan subRequest, 10),

		stop: make(chan bool, 1),

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
		router.wg.Add(1)
		for {
			if router.stopping && router.channelsAreEmpty() {
				router.closeRoutes()
				router.wg.Done()
				return
			}

			func() {
				defer protocol.PanicLogger()

				select {
				case message := <-router.handleC:
					router.handleMessage(message)
					runtime.Gosched()
				case subscriber := <-router.subscribeC:
					router.subscribe(subscriber.route)
					subscriber.doneNotify <- true
				case unsubscriber := <-router.unsubscribeC:
					router.unsubscribe(unsubscriber.route)
					unsubscriber.doneNotify <- true
				case <-router.stop:
					router.stopping = true
				}
			}()
		}
	}()

	return nil
}

// Stop stops the router by closing the stop channel
func (router *router) Stop() error {
	protocol.Debug("router: stopping")
	router.stop <- true
	router.wg.Wait()
	return nil
}

func (router *router) Check() error {
	err := router.messageStore.Check()
	if err != nil {
		return err
	}
	return nil
}

func (router *router) HandleMessage(message *protocol.Message) error {
	protocol.Debug("router: HandleMessage: %v %v", message.UserID, message.Path)
	if err := router.isStopping(); err != nil {
		return err
	}

	if !router.accessManager.IsAllowed(auth.WRITE, message.UserID, message.Path) {
		return &PermissionDeniedError{message.UserID, auth.WRITE, message.Path}
	}

	return router.storeMessage(message)
}

// Subscribe adds a route to the subscribers.
// If there is already a route with same Application Id and Path, it will be replaced.
func (router *router) Subscribe(r *Route) (*Route, error) {
	protocol.Debug("router: subscribe %v, %v, %v", router.accessManager, r.UserID, r.Path)

	if err := router.isStopping(); err != nil {
		return nil, err
	}

	accessAllowed := router.accessManager.IsAllowed(auth.READ, r.UserID, r.Path)
	if !accessAllowed {
		return r, &PermissionDeniedError{r.UserID, auth.READ, r.Path}
	}
	req := subRequest{
		route:      r,
		doneNotify: make(chan bool),
	}
	router.subscribeC <- req
	<-req.doneNotify
	return r, nil
}

func (router *router) Unsubscribe(r *Route) {
	req := subRequest{
		route:      r,
		doneNotify: make(chan bool),
	}
	router.unsubscribeC <- req
	<-req.doneNotify
}

func (router *router) subscribe(r *Route) {
	protocol.Debug("router: subscribe applicationID=%v, path=%v", r.ApplicationID, r.Path)

	list, present := router.routes[r.Path]
	if present {
		// Try to remove, to avoid double subscriptions of the same app
		list = remove(list, r)
	} else {
		// Path not present yet. Initialize the slice
		list = make([]*Route, 0, 1)
		router.routes[r.Path] = list
	}
	router.routes[r.Path] = append(list, r)
}

func (router *router) unsubscribe(r *Route) {
	protocol.Debug("router: unsubscribe applicationID=%v, path=%v", r.ApplicationID, r.Path)

	list, present := router.routes[r.Path]
	if !present {
		return
	}
	router.routes[r.Path] = remove(list, r)
	if len(router.routes[r.Path]) == 0 {
		delete(router.routes, r.Path)
	}
}
func (router *router) channelsAreEmpty() bool {
	return len(router.handleC) == 0 && len(router.subscribeC) == 0 && len(router.unsubscribeC) == 0
}

func (router *router) isStopping() error {
	if router.stopping {
		return &ModuleStoppingError{"Router"}
	}
	return nil
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
		protocol.Err("router: error storing message in partition %v: %v", msg.Path.Partition(), err)
		return err
	}

	if float32(len(router.handleC))/float32(cap(router.handleC)) > 0.9 {
		protocol.Warn("router: messageIn channel almost full: current length=%v, max. capacity=%v\n",
			len(router.handleC), cap(router.handleC))
		time.Sleep(time.Millisecond)
	}

	router.handleC <- msg
	return nil
}

func (router *router) handleMessage(message *protocol.Message) {
	protocol.Debug("router: routing message: %v", message.Metadata())

	for path, list := range router.routes {
		if matchesTopic(message.Path, path) {
			for _, route := range list {
				router.deliverMessage(route, message)
			}
		}
	}
}

func (router *router) deliverMessage(route *Route, message *protocol.Message) {
	defer protocol.PanicLogger()

	select {
	case route.MessagesChannel() <- &MessageForRoute{Message: message, Route: route}:
	// fine, we could send the message
	default:
		protocol.Warn("router: queue was full, closing delivery for route=%v to applicationID=%v", route.Path, route.ApplicationID)
		route.Close()
		router.unsubscribe(route)
	}
}

func (router *router) closeRoutes() {
	for _, currentRouteList := range router.routes {
		for _, route := range currentRouteList {
			route.Close()
			router.unsubscribe(route)
		}
	}
}

func copyOf(message []byte) []byte {
	messageCopy := make([]byte, len(message))
	copy(messageCopy, message)
	return messageCopy
}

// Test whether the supplied routePath matches the message topic
func matchesTopic(messagePath, routePath protocol.Path) bool {
	messagePathLen := len(string(messagePath))
	routePathLen := len(string(routePath))
	return strings.HasPrefix(string(messagePath), string(routePath)) &&
		(messagePathLen == routePathLen ||
			(messagePathLen > routePathLen && string(messagePath)[routePathLen] == '/'))
}

// remove a route from the supplied list,
// based on same ApplicationID id and same path
func remove(slice []*Route, route *Route) []*Route {
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
