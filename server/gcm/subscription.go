package gcm

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Bogh/gcm"
	log "github.com/Sirupsen/logrus"

	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/router"
	"github.com/smancke/guble/server/store"
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
	errSubReplaced        = errors.New("Subscription replaced")
	errIgnoreMessage      = errors.New("Message ignored")
	errSubscriptionExists = errors.New("Subscription exists")
)

type jsonError struct {
	json string
}

func (e *jsonError) Error() string {
	return e.json
}

// subscription represent a GCM subscription
type subscription struct {
	connector *Connector
	route     *router.Route
	lastID    uint64 // Last sent message id

	logger *log.Entry
}

// Creates a subscription and returns the pointer
func newSubscription(connector *Connector, route *router.Route, lastID uint64) *subscription {
	subLogger := logger.WithFields(log.Fields{
		"gcmID":  route.Get(applicationIDKey),
		"userID": route.Get(userIDKey),
		"topic":  string(route.Path),
		"lastID": lastID,
	})
	if connector.cluster != nil {
		subLogger = subLogger.WithField("nodeID", connector.cluster.Config.ID)
	}

	return &subscription{
		connector: connector,
		route:     route,
		lastID:    lastID,
		logger:    subLogger,
	}
}

// initSubscription creates a subscription and adds it in router/kvstore then starts listening for messages
func initSubscription(connector *Connector, topic, userID, gcmID string, lastID uint64, store bool) (*subscription, error) {
	route := router.NewRoute(router.RouteConfig{
		RouteParams: router.RouteParams{userIDKey: userID, applicationIDKey: gcmID},
		Path:        protocol.Path(topic),
		ChannelSize: subBufferSize,
		Matcher:     subscriptionMatcher,
	})

	s := newSubscription(connector, route, lastID)
	if s.exists() {
		return nil, errSubscriptionExists
	}

	// add subscription to gcm map
	s.connector.subscriptions[s.Key()] = s

	s.logger.Debug("New subscription")
	if store {
		if err := s.store(); err != nil {
			return nil, err
		}
	}

	return s, s.restart()
}

func subscriptionMatcher(route, other router.RouteConfig, keys ...string) bool {
	return route.Path == other.Path && route.Get(applicationIDKey) == other.Get(applicationIDKey)
}

// exists returns true if the subscription is present with the same key in gcm.subscriptions
func (s *subscription) exists() bool {
	_, ok := s.connector.subscriptions[s.Key()]
	return ok
}

func (s *subscription) subscribe() error {
	if _, err := s.connector.router.Subscribe(s.route); err != nil {
		s.logger.WithError(err).Error("Error subscribing in router")
		return err
	}

	s.logger.Debug("Subscribed")
	return nil
}

// unsubscribe from router and remove KVStore
func (s *subscription) remove() *subscription {
	s.logger.Info("Removing subscription")
	s.connector.router.Unsubscribe(s.route)
	delete(s.connector.subscriptions, s.Key())
	s.connector.kvStore.Delete(schema, s.Key())
	return s
}

// start loop to receive messages from route
func (s *subscription) start() error {
	if err := s.subscribe(); err != nil {
		return err
	}
	go s.subscriptionLoop()
	return nil
}

// recreate the route and resubscribe
func (s *subscription) restart() error {
	s.route = router.NewRoute(router.RouteConfig{
		RouteParams: s.route.RouteParams,
		Path:        s.route.Path,
		ChannelSize: subBufferSize,
	})

	// fetch until we reach the end
	for s.shouldFetch() {
		// fetch from last id if applicable
		if err := s.fetch(); err != nil {
			return err
		}
	}

	// subscribe to the router and start
	return s.start()
}

// returns true if we should continue fetching
// checks if lastID has not reached maxMessageID from partition
func (s *subscription) shouldFetch() bool {
	if s.lastID <= 0 {
		return false
	}

	messageStore, err := s.connector.router.MessageStore()
	if err != nil {
		s.logger.WithField("error", err).Error("Error retrieving message store instance from router")
		return false
	}

	maxID, err := messageStore.MaxMessageID(s.route.Path.Partition())
	if err != nil {
		s.logger.WithField("error", err).Error("Error retrieving max message ID")
		return false
	}

	return s.lastID < maxID
}

// subscriptionLoop that will run in a goroutine and pipe messages from route to gcm
// Attention: in order for this loop to finish the route channel must stop sending messages
func (s *subscription) subscriptionLoop() {
	s.logger.Debug("Starting subscription loop")

	s.connector.wg.Add(1)
	defer func() {
		s.logger.Debug("Stopped subscription loop")
		s.connector.wg.Done()
	}()

	var (
		m      *protocol.Message
		opened = true
	)
	for opened {
		select {
		case m, opened = <-s.route.MessagesChannel():
			// TODO Bogdan This needs to be remade and we should gracefully shutdown
			// and wait for this channel to empty before stopping the loop
			select {
			case <-s.connector.stopC:
				return
			case <-time.After(10 * time.Millisecond):
			}

			if !opened {
				s.logger.Error("Route channel is closed")
				continue
			}

			if err := s.pipe(m); err != nil {
				// abbandon route if the following 2 errors are met
				// the subscription has been replaced
				if err == errSubReplaced {
					return
				}
				// the subscription is not registered with GCM anymore
				if _, ok := err.(*jsonError); ok {
					return
				}

				s.logger.WithError(err).Error("Error pipelining message")
			}
		case <-s.connector.stopC:
			return
		}
	}

	// if route is closed and we are actually stopping then return
	if s.isStopping() {
		return
	}

	// assume that the route channel has been closed because of slow processing
	// try restarting, by fetching from lastId and then subscribing again
	if err := s.restart(); err != nil {
		if stoppingErr, ok := err.(*router.ModuleStoppingError); ok {
			s.logger.WithField("error", stoppingErr).Debug("Error restarting subscription")
		}
	}
}

// fetch messages from store starting with lastID
func (s *subscription) fetch() error {
	s.connector.wg.Add(1)
	defer func() {
		s.logger.WithField("lastID", s.lastID).Debug("Stop fetching")
		s.connector.wg.Done()
	}()

	s.logger.WithField("lastID", s.lastID).Debug("Fetching from store")
	req := s.createFetchRequest()
	if err := s.connector.router.Fetch(req); err != nil {
		return err
	}

	for {
		select {
		case results := <-req.StartC:
			s.logger.WithField("count", results).Debug("Receiving count")
		case msgAndID, open := <-req.MessageC:
			if !open {
				s.logger.Debug("Fetch channel closed.")
				return nil
			}
			message, err := protocol.ParseMessage(msgAndID.Message)
			if err != nil {
				return err
			}
			s.logger.WithFields(log.Fields{"ID": msgAndID.ID, "parsedID": message.ID}).Debug("Fetched message")

			// Pipe message into gcm connector
			s.pipe(message)
		case err := <-req.ErrorC:
			return err
		case <-s.connector.stopC:
			s.logger.Debug("Stopping fetch because service is shutting down")
			return nil
		}
	}
}

func (s *subscription) createFetchRequest() store.FetchRequest {
	return store.FetchRequest{
		Partition: s.route.Path.Partition(),
		StartID:   s.lastID + 1,
		Direction: 1,
		Count:     math.MaxInt32,
		MessageC:  make(chan store.FetchedMessage, 5),
		ErrorC:    make(chan error),
		StartC:    make(chan int),
	}
}

// isStopping returns true if we are actually stopping the service
func (s *subscription) isStopping() bool {
	select {
	case <-s.connector.stopC:
		return true
	default:
	}
	return false
}

// bytes data to store in kvStore
func (s *subscription) bytes() []byte {
	return []byte(strings.Join([]string{
		s.route.Get(userIDKey),
		string(s.route.Path),
		strconv.FormatUint(s.lastID, 10),
	}, ":"))
}

// store data in kvstore
func (s *subscription) store() error {
	s.logger.WithField("lastID", s.lastID).Debug("Storing subscription")
	err := s.connector.kvStore.Put(schema, s.Key(), s.bytes())
	if err != nil {
		s.logger.WithError(err).Error("Error storing in KVStore")
	}

	return err
}

// Key returns a string that uniquely identifies this subscription
func (s *subscription) Key() string {
	return s.route.Key()
}

func (s *subscription) setLastID(ID uint64) error {
	s.lastID = ID
	// update KV when last id is set
	return s.store()
}

// pipe sends a message into the pipeline and waits for response saving the last id in the kvstore
func (s *subscription) pipe(m *protocol.Message) error {
	pipeMessage := newPipeMessage(s, m)
	defer pipeMessage.closeChannels()

	s.connector.pipelineC <- pipeMessage

	// wait for response
	select {
	case response := <-pipeMessage.resultC:
		if err := s.setLastID(pipeMessage.message.ID); err != nil {
			return err
		}
		return s.handleGCMResponse(response)
	case err := <-pipeMessage.errC:
		if err == errIgnoreMessage {
			s.logger.WithField("message", m).Info("Ignoring message")
			return nil
		}
		s.logger.WithError(err).Error("Error sending message to GCM")
		return err
	}
}

func (s *subscription) handleGCMResponse(response *gcm.Response) error {
	if response.Ok() {
		return nil
	}

	if err := s.handleJSONError(response); err != nil {
		return err
	}

	s.logger.WithField("count", response.Success).Debug("Delivered messages to GCM")
	if response.CanonicalIDs != 0 {
		// we only send to one receiver,
		// so we know that we can replace the old id with the first registration id (=canonical id)
		return s.replaceCanonical(response.Results[0].RegistrationID)
	}
	return nil
}

func (s *subscription) handleJSONError(response *gcm.Response) error {
	err := response.Error
	errText := err.Error()

	if errText != "" {
		if errText == "NotRegistered" {
			s.logger.Debug("Removing not registered GCM subscription")
			s.remove()
			return &jsonError{errText}
		} else if errText == "InvalidRegistration" {
			s.logger.WithField("jsonError", errText).Error("Subscription is not registered")
		} else {
			s.logger.WithField("jsonError", errText).Error("Unexpected error while sending to GCM")
		}
	}
	return nil
}

// isValidResponseError returns True if the error is accepted as a valid response
// cases are InvalidRegistration and NotRegistered
func (s *subscription) isValidResponseError(err error) bool {
	return err.Error() == "InvalidRegistration" ||
		err.Error() == "NotRegistered"
}

// replaceCanonical replaces subscription with canonical id,
// creates a new subscription but alters the route to have the new ApplicationID
func (s *subscription) replaceCanonical(newGCMID string) error {
	s.logger.WithField("newGCMID", newGCMID).Info("Replacing with canonicalID")
	// delete current route from kvstore
	s.remove()

	// reuse the route but change the ApplicationID
	route := s.route
	route.Set(applicationIDKey, newGCMID)
	newS := newSubscription(s.connector, route, s.lastID)

	if err := newS.store(); err != nil {
		return err
	}
	newS.start()
	return errSubReplaced
}
