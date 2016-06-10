package gcm

import (
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/alexjlockwood/gcm"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server"
	"strconv"
	"strings"
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

// Creates a subscription and returns the pointer
func newSub(gcm *Connector, route *server.Route, lastID uint64) *sub {
	return &sub{
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
func initSub(gcm *Connector, topic, userID, gcmID string, lastID uint64) (*sub, error) {
	route := server.NewRoute(topic, gcmID, userID, subBufferSize)
	s := newSub(gcm, route, 0)
	if err := s.store(); err != nil {
		return nil, err
	}
	s.start()
	return s, nil
}

// sub represent a GCM subscription
type sub struct {
	gcm    *Connector
	route  *server.Route
	lastID uint64 // Last sent message id

	logger *log.Entry
}

func (s *sub) subscribe() error {
	if _, err := s.gcm.router.Subscribe(s.route); err != nil {
		s.logger.WithField("error", err).Error("Error subscribing")
		return err
	}

	s.logger.Debug("Subscribed")
	return nil
}

// unsubscribe from router and remove KVStore
func (s *sub) remove() *sub {
	s.gcm.router.Unsubscribe(s.route)
	s.gcm.kvStore.Delete(schema, s.route.ApplicationID)
	return s
}

// start loop to receive messages from route
func (s *sub) start() error {
	if err := s.subscribe(); err != nil {
		return err
	}
	go s.subscriptionLoop()
	return nil
}

// recreate the route and resubscribe
func (s *sub) restart() error {
	s.route = server.NewRoute(string(s.route.Path), s.route.ApplicationID, s.route.UserID, subBufferSize)

	// fetch from last id if applicable
	if err := s.fetch(); err != nil {
		return err
	}

	// subscribe to the router and start
	if err := s.subscribe(); err != nil {
		return err
	}

	s.start()
	return nil
}

// subscriptionLoop that will run in a goroutine and pipe messages from route to gcm
// Attention: in order for this loop to finish the route channel must be closed
func (s *sub) subscriptionLoop() {
	s.logger.Debug("Starting subscription loop")

	s.gcm.wg.Add(1)
	defer func() {
		s.logger.Debug("Stopped subscription loop")
		s.gcm.wg.Done()
	}()

	// no need to wait for `*gcm.stopC` the channel will be closed by the router anyway
	for m := range s.route.MessagesChannel() {
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

			s.logger.WithField("error", err).Error("Error pipelining message")
		}
	}
	// if route is closed and we are actually stopping then return
	if s.isStopping() {
		return
	}

	// assume that the route channel has been closed cause of slow processing
	// reconnect. try restarting

}

// fetch messages from store starting with lastID
func (s *sub) fetch() error {

}

// returns true if we are actually stopping the service
func (s *sub) isStopping() bool {
	select {
	case <-s.gcm.stopC:
		return true
	default:
	}

	return false
}

// return bytes data to store in kvStore
func (s *sub) bytes() []byte {
	return []byte(strings.Join([]string{
		s.route.UserID,
		string(s.route.Path),
		strconv.FormatUint(s.lastID, 10),
	}, ":"))
}

// store data in kvstore
func (s *sub) store() error {
	err := s.gcm.kvStore.Put(schema, s.route.ApplicationID, s.bytes())
	if err != nil {
		s.logger.WithField("error", err).Error("Error storing in KVStore")
	}
	return err
}

func (s *sub) setLastID(ID uint64) error {
	s.lastID = ID
	// update KV when last id is set
	return s.store()
}

// sends a message into the pipeline and waits for response saving the last id
// in the kvstore
func (s *sub) pipe(m *protocol.Message) error {
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
		s.logger.WithField("error", err).Error("Error sending message to GCM")
		return err
	}

	return nil
}

func (s *sub) handleGCMResponse(response *gcm.Response) error {
	if err := s.handleJSONError(response); err != nil {
		return err
	}

	s.logger.Debug("Delivered message to GCM")
	if response.CanonicalIDs != 0 {
		// we only send to one receiver,
		// so we know that we can replace the old id with the first registration id (=canonical id)
		return s.replaceCanonical(response.Results[0].RegistrationID)
	}
	return nil
}

func (s *sub) handleJSONError(response *gcm.Response) error {
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
func (s *sub) replaceCanonical(newGCMID string) error {
	s.logger.WithField("newGCMID", newGCMID).Info("Replacing with canonicalID")

	// delete current route from kvstore
	s.remove()

	// reuse the route but change the ApplicationID
	route := s.route
	route.ApplicationID = newGCMID
	newS := newSub(s.gcm, route, s.lastID)

	if err := newS.store(); err != nil {
		return err
	}
	newS.start()
	return errSubReplaced
}

func newPipeMessage(s *sub, m *protocol.Message) *pipeMessage {
	return &pipeMessage{s, m, make(chan *gcm.Response, 1), make(chan error, 1)}
}

// Pipeline message
type pipeMessage struct {
	*sub
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
