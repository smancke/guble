package server

import (
	"fmt"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/auth"
	"github.com/smancke/guble/store"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	overloadedHandleChannelRatio = 0.9
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
	routes       map[protocol.Path][]*Route // mapping the path to the route slice
	handleC      chan *protocol.Message
	subscribeC   chan subRequest
	unsubscribeC chan subRequest
	stopC        chan bool      // Channel that signals stop of the router
	stopping     bool           // Flag: the router is in stopping process and no incoming messages are accepted
	wg           sync.WaitGroup // Add any operation that we need to wait upon here

	accessManager auth.AccessManager
	messageStore  store.MessageStore
	kvStore       store.KVStore
}

// NewRouter returns a pointer to Router
func NewRouter(accessManager auth.AccessManager, messageStore store.MessageStore, kvStore store.KVStore) Router {
	return &router{
		routes: make(map[protocol.Path][]*Route),

		handleC:      make(chan *protocol.Message, 500),
		subscribeC:   make(chan subRequest, 10),
		unsubscribeC: make(chan subRequest, 10),
		stopC:        make(chan bool, 1),

		accessManager: accessManager,
		messageStore:  messageStore,
		kvStore:       kvStore,
	}
}

func (router *router) Start() error {
	router.panicIfInternalDependenciesAreNil()
	resetMetricsRouter()
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
					router.routeMessage(message)
					runtime.Gosched()
				case subscriber := <-router.subscribeC:
					router.subscribe(subscriber.route)
					subscriber.doneNotify <- true
				case unsubscriber := <-router.unsubscribeC:
					router.unsubscribe(unsubscriber.route)
					unsubscriber.doneNotify <- true
				case <-router.stopC:
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
	router.stopC <- true
	router.wg.Wait()
	return nil
}

func (router *router) Check() error {
	if router.accessManager == nil || router.messageStore == nil || router.kvStore == nil {
		return ErrServiceNotProvided
	}

	err := router.messageStore.Check()
	if err != nil {
		return err
	}

	err = router.kvStore.Check()
	if err != nil {
		return err
	}

	return nil
}

func (router *router) HandleMessage(message *protocol.Message) error {
	protocol.Debug("router: HandleMessage: %v %v", message.UserID, message.Path)
	mTotalMessagesIncoming.Add(1)

	if err := router.isStopping(); err != nil {
		return err
	}

	if !router.accessManager.IsAllowed(auth.WRITE, message.UserID, message.Path) {
		return &PermissionDeniedError{message.UserID, auth.WRITE, message.Path}
	}

	return router.storeAndChannelMessage(message)
}

// Subscribe adds a route to the subscribers.
// If there is already a route with same Application Id and Path, it will be replaced.
func (router *router) Subscribe(r *Route) (*Route, error) {
	protocol.Debug("router: Subscribe %v, %v, %v", router.accessManager, r.UserID, r.Path)

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
	protocol.Debug("router: Unsubscribe %v, %v, %v", router.accessManager, r.UserID, r.Path)

	req := subRequest{
		route:      r,
		doneNotify: make(chan bool),
	}
	router.unsubscribeC <- req
	<-req.doneNotify
}

func (router *router) subscribe(r *Route) {
	protocol.Debug("router: subscribe applicationID=%v, path=%v", r.ApplicationID, r.Path)
	mTotalSubscriptionAttempts.Add(1)

	slice, present := router.routes[r.Path]
	var removed bool
	if present {
		// Try to remove, to avoid double subscriptions of the same app
		slice, removed = removeIfMatching(slice, r)
	} else {
		// Path not present yet. Initialize the slice
		slice = make([]*Route, 0, 1)
		router.routes[r.Path] = slice
		mCurrentRoutes.Add(1)
	}
	router.routes[r.Path] = append(slice, r)
	if removed {
		mTotalDuplicateSubscriptionsAttempts.Add(1)
	} else {
		mTotalSubscriptions.Add(1)
		mCurrentSubscriptions.Add(1)
	}
}

func (router *router) unsubscribe(r *Route) {
	protocol.Debug("router: unsubscribe applicationID=%v, path=%v", r.ApplicationID, r.Path)
	mTotalUnsubscriptionAttempts.Add(1)

	slice, present := router.routes[r.Path]
	if !present {
		mTotalInvalidTopicOnUnsubscriptionAttempts.Add(1)
		return
	}
	var removed bool
	router.routes[r.Path], removed = removeIfMatching(slice, r)
	if removed {
		mTotalUnsubscriptions.Add(1)
		mCurrentSubscriptions.Add(-1)
	} else {
		mTotalInvalidUnsubscriptionAttempts.Add(1)
	}
	if len(router.routes[r.Path]) == 0 {
		delete(router.routes, r.Path)
		mCurrentRoutes.Add(-1)
	}
}

func (router *router) panicIfInternalDependenciesAreNil() {
	if router.accessManager == nil || router.kvStore == nil || router.messageStore == nil {
		panic(fmt.Sprintf("router: the internal dependencies marked with `true` are not set: AccessManager=%v, KVStore=%v, MessageStore=%v",
			router.accessManager == nil, router.kvStore == nil, router.messageStore == nil))
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

// storeAndChannelMessage assigns the new message id, stores message
// and handle it by passing the stored message to the handleC channel
func (router *router) storeAndChannelMessage(msg *protocol.Message) error {
	txCallback := func(msgId uint64) []byte {
		msg.ID = msgId
		msg.Time = time.Now().Unix()
		return msg.Bytes()
	}

	if err := router.messageStore.StoreTx(msg.Path.Partition(), txCallback); err != nil {
		protocol.Err("router: error storing message in partition %v: %v", msg.Path.Partition(), err)
		mTotalMessageStoreErrors.Add(1)
		return err
	}

	if float32(len(router.handleC))/float32(cap(router.handleC)) > overloadedHandleChannelRatio {
		protocol.Warn("router: handleC channel almost full: current length=%v, max. capacity=%v",
			len(router.handleC), cap(router.handleC))
		mTotalOverloadedHandleChannel.Add(1)
	}

	router.handleC <- msg
	return nil
}

func (router *router) routeMessage(message *protocol.Message) {
	protocol.Debug("router: routeMessage: %v", message.Metadata())
	mTotalMessagesRouted.Add(1)

	for path, list := range router.routes {
		if matchesTopic(message.Path, path) {
			for _, route := range list {
				router.deliverMessage(route, message)
			}
		} else {
			mTotalMessagesNotMatchingTopic.Add(1)
		}
	}
}

func (router *router) deliverMessage(route *Route, message *protocol.Message) {
	defer protocol.PanicLogger()

	select {
	case route.MessagesChannel() <- &MessageForRoute{Message: message, Route: route}:
	// fine, we could send the message
	default:
		protocol.Warn("router: deliverMessage: route's channel was full, unsubscribing and closing delivery channel for route: %v", route)
		router.unsubscribe(route)
		route.Close()
		mTotalDeliverMessageErrors.Add(1)
	}
}

func (router *router) closeRoutes() {
	protocol.Debug("router: closeRoutes()")
	for _, currentRouteList := range router.routes {
		for _, route := range currentRouteList {
			router.unsubscribe(route)
			protocol.Debug("router: closing route %v", route)
			route.Close()
		}
	}
}

func copyOf(message []byte) []byte {
	messageCopy := make([]byte, len(message))
	copy(messageCopy, message)
	return messageCopy
}

// matchesTopic checks whether the supplied routePath matches the message topic
func matchesTopic(messagePath, routePath protocol.Path) bool {
	messagePathLen := len(string(messagePath))
	routePathLen := len(string(routePath))
	return strings.HasPrefix(string(messagePath), string(routePath)) &&
		(messagePathLen == routePathLen ||
			(messagePathLen > routePathLen && string(messagePath)[routePathLen] == '/'))
}

// remove removes a route from the supplied list, based on same ApplicationID id and same path (if existing)
// returns: the (possibly updated) slide, and a boolean value (true if route was removed, false otherwise)
func removeIfMatching(slice []*Route, route *Route) ([]*Route, bool) {
	position := -1
	for p, r := range slice {
		if r.ApplicationID == route.ApplicationID && r.Path == route.Path {
			position = p
		}
	}
	if position == -1 {
		return slice, false
	}
	return append(slice[:position], slice[position+1:]...), true
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
