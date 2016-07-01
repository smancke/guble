package gcm

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/alexjlockwood/gcm"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"
)

const (
	// default subscription channel buffer size
	subBufferSize = 50
)

var (
	errSubReplaced = errors.New("Subscription replaced")
)

type jsonError struct {
	json string
}

func (e *jsonError) Error() string {
	return e.json
}

// subscription represent a GCM subscription
type subscription struct {
	gcm    *Connector
	route  *server.Route
	lastID uint64 // Last sent message id

	logger *log.Entry
}

// Creates a subscription and returns the pointer
func newSubscription(gcm *Connector, route *server.Route, lastID uint64) *subscription {
	return &subscription{
		gcm:    gcm,
		route:  route,
		lastID: lastID,
		logger: logger.WithFields(log.Fields{
			"gcmID":  route.ApplicationID,
			"userID": route.UserID,
			"topic":  string(route.Path),
		}),
	}
}

// creates a subscription and adds it in router/kvstore then starts listening for messages
func initSubscription(gcm *Connector, topic, userID, gcmID string, lastID uint64) (*subscription, error) {
	route := server.NewRoute(topic, gcmID, userID, subBufferSize)
	s := newSubscription(gcm, route, 0)
	s.logger.Debug("New subscription")
	if err := s.store(); err != nil {
		return nil, err
	}
	s.restart()
	return s, nil
}

func (s *subscription) subscribe() error {
	if _, err := s.gcm.router.Subscribe(s.route); err != nil {
		s.logger.WithError(err).Error("Error subscribing")
		return err
	}
	s.logger.Debug("Subscribed")
	return nil
}

// unsubscribe from router and remove KVStore
func (s *subscription) remove() *subscription {
	s.gcm.router.Unsubscribe(s.route)
	s.gcm.kvStore.Delete(schema, s.route.ApplicationID)
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
	s.route = server.NewRoute(string(s.route.Path), s.route.ApplicationID, s.route.UserID, subBufferSize)

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

	messageStore, err := s.gcm.router.MessageStore()
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
// Attention: in order for this loop to finish the route channel must be closed
func (s *subscription) subscriptionLoop() {
	s.logger.Debug("Starting subscription loop")

	s.gcm.wg.Add(1)
	defer func() {
		s.logger.Debug("Stopped subscription loop")
		s.gcm.wg.Done()
	}()

	var (
		m      *protocol.Message
		opened = true
	)
	for opened {
		select {
		case m, opened = <-s.route.MessagesChannel():
			if !opened {
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
		case <-s.gcm.stopC:
			return
		}
	}

	// if route is closed and we are actually stopping then return
	if s.isStopping() {
		return
	}

	// assume that the route channel has been closed cause of slow processing
	// try restarting, by fetching from lastId and then subscribing again
	if err := s.restart(); err != nil {
		if stoppingErr, ok := err.(*server.ModuleStoppingError); ok {
			s.logger.WithField("error", stoppingErr).Debug("Error restarting subscription")
		}
	}
}

// fetch messages from store starting with lastID
func (s *subscription) fetch() error {
	// if s.lastID == 0 {
	// 	s.logger.WithField("lastID", s.lastID).Debug("Nothing to fetch")
	// 	return nil
	// }

	s.gcm.wg.Add(1)
	defer func() {
		s.logger.WithField("lastID", s.lastID).Debug("Stop fetching")
		s.gcm.wg.Done()
	}()

	s.logger.Debug("Fetching from store")
	req := s.createFetchRequest()
	if err := s.gcm.router.Fetch(req); err != nil {
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
		case <-s.gcm.stopC:
			s.logger.Debug("Stopping fetch cause service is shutting down")
			return nil
		}
	}
	return nil
}

func (s *subscription) createFetchRequest() store.FetchRequest {
	return store.FetchRequest{
		Partition: s.route.Path.Partition(),
		StartID:   s.lastID + 1,
		Direction: 1,
		Count:     math.MaxInt32,
		MessageC:  make(chan store.MessageAndID, 5),
		ErrorC:    make(chan error),
		StartC:    make(chan int),
	}
}

// returns true if we are actually stopping the service
func (s *subscription) isStopping() bool {
	select {
	case <-s.gcm.stopC:
		return true
	default:
	}

	return false
}

// return bytes data to store in kvStore
func (s *subscription) bytes() []byte {
	return []byte(strings.Join([]string{
		s.route.UserID,
		string(s.route.Path),
		strconv.FormatUint(s.lastID, 10),
	}, ":"))
}

// store data in kvstore
func (s *subscription) store() error {
	s.logger.WithField("lastID", s.lastID).Debug("Storing subscription")
	err := s.gcm.kvStore.Put(schema, s.route.ApplicationID, s.bytes())
	if err != nil {
		s.logger.WithError(err).Error("Error storing in KVStore")
	}
	return err
}

func (s *subscription) setLastID(ID uint64) error {
	s.lastID = ID
	// update KV when last id is set
	return s.store()
}

// sends a message into the pipeline and waits for response saving the last id
// in the kvstore
func (s *subscription) pipe(m *protocol.Message) error {
	pm := newPipeMessage(s, m)
	defer pm.close()

	// send pipeMessage into pipeline
	s.gcm.pipelineC <- pm

	// wait for response
	select {
	case response := <-pm.resultC:
		if err := s.setLastID(pm.message.ID); err != nil {
			return err
		}
		return s.handleGCMResponse(response)
	case err := <-pm.errC:
		s.logger.WithError(err).Error("Error sending message to GCM")
		return err
	}
}

func (s *subscription) handleGCMResponse(response *gcm.Response) error {
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
	errText := response.Results[0].Error
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

// replace subscription with cannoical id, creates a new subscription but alters the route to
// have the new ApplicationID
func (s *subscription) replaceCanonical(newGCMID string) error {
	s.logger.WithField("newGCMID", newGCMID).Info("Replacing with canonicalID")

	// delete current route from kvstore
	s.remove()

	// reuse the route but change the ApplicationID
	route := s.route
	route.ApplicationID = newGCMID
	newS := newSubscription(s.gcm, route, s.lastID)

	if err := newS.store(); err != nil {
		return err
	}
	newS.start()
	return errSubReplaced
}

func newPipeMessage(s *subscription, m *protocol.Message) *pipeMessage {
	return &pipeMessage{s, m, make(chan *gcm.Response, 1), make(chan error, 1)}
}

// Pipeline message
type pipeMessage struct {
	*subscription
	message *protocol.Message
	resultC chan *gcm.Response
	errC    chan error
}

func (pm *pipeMessage) payload() map[string]interface{} {
	return messageMap(pm.message)
}

func (pm *pipeMessage) close() {
	close(pm.resultC)
	close(pm.errC)
}

func messageMap(m *protocol.Message) map[string]interface{} {
	payload := make(map[string]interface{})

	if m.Body[0] == '{' {
		json.Unmarshal(m.Body, &payload)
	} else {
		payload["message"] = m.BodyAsString()
	}

	return payload
}
