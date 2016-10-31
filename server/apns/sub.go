package apns

import (
	"context"
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/router"
	"github.com/smancke/guble/server/store"
	"strconv"
	"strings"
)

const (
	// default subscription channel buffer size
	subBufferSize = 50

	// applicationIDKey is the key name set on the route params to identify the application
	applicationIDKey = "device_id"

	// userIDKey is the key name set on the route params to identify the user
	userIDKey = "user_id"
)

var (
	errSubscriptionExists = errors.New("Subscription exists")
)

// subscription represents a APNS subscription
type sub struct {
	connector *Connector
	route     *router.Route
	lastID    uint64 // Last sent message id

	logger *log.Entry
}

// initSubscription creates a subscription and adds it in router/kvstore then starts listening for messages
func initSubscription(connector *Connector, topic, userID, apnsDeviceID string, lastID uint64, store bool) (*sub, error) {
	route := router.NewRoute(router.RouteConfig{
		RouteParams: router.RouteParams{userIDKey: userID, applicationIDKey: apnsDeviceID},
		Path:        protocol.Path(topic),
		ChannelSize: subBufferSize,
		Matcher:     subscriptionMatcher,
	})

	s := newSubscription(connector, route, lastID)
	if s.exists() {
		return nil, errSubscriptionExists
	}

	// add subscription to map
	s.connector.subs[s.Key()] = s

	s.logger.Debug("New subscription")
	if store {
		if err := s.store(); err != nil {
			return nil, err
		}
	}

	return s, s.restart(connector.context)
}

func subscriptionMatcher(route, other router.RouteConfig, keys ...string) bool {
	return route.Path == other.Path && route.Get(applicationIDKey) == other.Get(applicationIDKey)
}

// newSubscription creates a subscription and returns the pointer
func newSubscription(connector *Connector, route *router.Route, lastID uint64) *sub {
	subLogger := logger.WithFields(log.Fields{
		"fcmID":  route.Get(applicationIDKey),
		"userID": route.Get(userIDKey),
		"topic":  string(route.Path),
		"lastID": lastID,
	})

	return &sub{
		connector: connector,
		route:     route,
		lastID:    lastID,
		logger:    subLogger,
	}
}

// exists returns true if the subscription is present with the same key in subscriptions map
func (s *sub) exists() bool {
	_, ok := s.connector.subs[s.Key()]
	return ok
}

// restart recreates the route and resubscribes
func (s *sub) restart(ctx context.Context) error {
	s.route = router.NewRoute(router.RouteConfig{
		RouteParams: s.route.RouteParams,
		Path:        s.route.Path,
		ChannelSize: subBufferSize,
	})

	// subscribe to the router and start the loop
	return s.start(ctx)
}

// start loop to receive messages from route
func (s *sub) start(ctx context.Context) error {
	s.route.FetchRequest = s.createFetchRequest()
	go s.Loop(ctx)
	if err := s.route.Provide(s.connector.router, true); err != nil {
		return err
	}
	return nil
}

func (s *sub) createFetchRequest() *store.FetchRequest {
	if s.lastID <= 0 {
		return nil
	}
	return store.NewFetchRequest("", s.lastID+1, 0, store.DirectionForward, -1)
}

// subscriptionLoop that will run in a goroutine and pipe messages from route to fcm
// Attention: in order for this loop to finish the route channel must stop sending messages
func (s *sub) Loop(ctx context.Context) {
	s.logger.Debug("Starting APNS subscription loop")

	var (
		m      *protocol.Message
		opened = true
	)
	for opened {
		select {
		case <-ctx.Done():
			return
		case m, opened = <-s.route.MessagesChannel():
			s.push(m)
		}
	}

	// assume that the route channel has been closed because of slow processing
	// try restarting, by fetching from lastId and then subscribing again
	if err := s.restart(ctx); err != nil {
		if stoppingErr, ok := err.(*router.ModuleStoppingError); ok {
			s.logger.WithField("error", stoppingErr).Debug("Error restarting subscription")
		}
	}
}

// Key returns a string that uniquely identifies this subscription
func (s *sub) Key() string {
	return s.route.Key()
}

// push a message into the queue
func (s *sub) push(m *protocol.Message) {

	//TODO Cosmin: Samsa should generate the Payload or the whole Notification, and JSON-serialize it into the guble-message Body.

	//n := &apns2.Notification{
	//	Priority:    apns2.PriorityHigh,
	//	Topic:       strings.TrimPrefix(string(s.route.Path), "/"),
	//	DeviceToken: s.route.Get(applicationIDKey),
	//	Payload:     m.Body,
	//}

	n := &apns2.Notification{
		Priority:    apns2.PriorityHigh,
		Topic:       strings.TrimPrefix(string(s.route.Path), "/"),
		DeviceToken: s.route.Get(applicationIDKey),
		Payload: payload.NewPayload().
			AlertTitle("Title").
			AlertBody("Text").
			Badge(1).
			ContentAvailable(),
	}

	s.connector.queue.push(n, m, s)
}

func (s *sub) setLastID(ID uint64) error {
	s.lastID = ID
	// update KV when last id is set
	return s.store()
}

// store data in kvstore
func (s *sub) store() error {
	s.logger.WithField("lastID", s.lastID).Debug("Storing subscription")
	applicationID := s.route.Get(applicationIDKey)
	err := s.connector.kvStore.Put(schema, applicationID, s.bytes())
	if err != nil {
		s.logger.WithError(err).Error("Error storing in KVStore")
	}
	return err
}

// bytes returns the data to store in kvStore
func (s *sub) bytes() []byte {
	return []byte(strings.Join([]string{
		s.route.Get(userIDKey),
		string(s.route.Path),
		strconv.FormatUint(s.lastID, 10),
	}, ":"))
}

// remove unsubscribes from router, delete from connector's subscriptions, and remove from KVStore
func (s *sub) remove() *sub {
	s.logger.Debug("Removing subscription")
	s.connector.router.Unsubscribe(s.route)
	delete(s.connector.subs, s.Key())
	s.connector.kvStore.Delete(schema, s.Key())
	return s
}
