package fcm

import (
	"errors"
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

// subscription represents a FCM subscription
type subscription struct {
	connector *Connector
	route     *router.Route
	lastID    uint64 // Last sent message id

	logger *log.Entry
}

// Creates a subscription and returns the pointer
func newSubscription(connector *Connector, route *router.Route, lastID uint64) *subscription {
	subLogger := logger.WithFields(log.Fields{
		"fcmID":  route.Get(applicationIDKey),
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
func initSubscription(connector *Connector, topic, userID, fcmID string, lastID uint64, store bool) (*subscription, error) {
	route := router.NewRoute(router.RouteConfig{
		RouteParams: router.RouteParams{userIDKey: userID, applicationIDKey: fcmID},
		Path:        protocol.Path(topic),
		ChannelSize: subBufferSize,
		Matcher:     subscriptionMatcher,
	})

	s := newSubscription(connector, route, lastID)
	if s.exists() {
		return nil, errSubscriptionExists
	}

	// add subscription to map
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

// exists returns true if the subscription is present with the same key in subscriptions map
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
	s.route.FetchRequest = s.createFetchRequest()
	go s.subscriptionLoop()
	if err := s.route.Provide(s.connector.router, true); err != nil {
		return err
	}
	return nil
}

// recreate the route and resubscribe
func (s *subscription) restart() error {
	s.route = router.NewRoute(router.RouteConfig{
		RouteParams: s.route.RouteParams,
		Path:        s.route.Path,
		ChannelSize: subBufferSize,
	})

	// subscribe to the router and start the loop
	return s.start()
}

func (s *subscription) createFetchRequest() *store.FetchRequest {
	if s.lastID <= 0 {
		return nil
	}
	return store.NewFetchRequest("", s.lastID+1, 0, store.DirectionForward, -1)
}

// subscriptionLoop that will run in a goroutine and pipe messages from route to fcm
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
			case <-time.After(5 * time.Millisecond):
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
				// the subscription is not registered with FCM anymore
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
	applicationID := s.route.Get(applicationIDKey)
	err := s.connector.kvStore.Put(schema, applicationID, s.bytes())
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
		s.logger.WithField("messageID", m.ID).Debug("Delivered message to FCM")
		if err := s.setLastID(pipeMessage.message.ID); err != nil {
			return err
		}
		return s.handleFCMResponse(response)
	case err := <-pipeMessage.errC:
		if err == errIgnoreMessage {
			s.logger.WithField("message", m).Info("Ignoring message")
			return nil
		}
		s.logger.WithField("error", err.Error()).Error("Error sending message to FCM")
		return err
	}
}

func (s *subscription) handleFCMResponse(response *gcm.Response) error {
	if response.Ok() {
		return nil
	}

	s.logger.WithField("count", response.Success).Debug("Handling FCM Error")
	if err := s.handleJSONError(response); err != nil {
		return err
	}

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
func (s *subscription) replaceCanonical(newFCMID string) error {
	s.logger.WithField("newFCMID", newFCMID).Info("Replacing with FCM canonicalID")
	// delete current route from kvstore
	s.remove()

	// reuse the route but change the ApplicationID
	route := s.route
	route.Set(applicationIDKey, newFCMID)
	newSub := newSubscription(s.connector, route, s.lastID)

	if err := newSub.store(); err != nil {
		return err
	}
	newSub.start()
	return errSubReplaced
}
