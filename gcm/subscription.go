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
func newSub(gcm *GCMConnector, route *server.Route, lastID uint64) *sub {
	return &sub{
		gcm:    gcm,
		route:  route,
		lastID: lastID,
		logger: logger.WithFields(log.Fields{
			"gcm_id":  route.ApplicationID,
			"user_id": route.UserID,
			"topic":   string(route.Path),
		}),
	}
}

// creates a subscription and adds it in router/kvstore then starts listening for messages
func initSub(gcm *GCMConnector, topic, userID, gcmID string, lastID uint64) *sub {
	route := server.NewRoute(topic, gcmID, userID, subBufferSize)
	return newSub(gcm, route, 0).add().start()
}

// sub represent a GCM subscription
type sub struct {
	gcm    *GCMConnector
	route  *server.Route
	lastID uint64 // Last sent message id

	logger *log.Entry
}

// subscribe to router and add the subscription to GCM subs list
func (s *sub) add() *sub {
	s.logger.Debug("Subscribed")

	s.gcm.router.Subscribe(s.route)
	s.store()
	return s
}

// unsubscribe from router and remove KVStore
func (s *sub) remove() *sub {
	s.gcm.router.Unsubscribe(s.route)
	s.gcm.kvStore.Delete(schema, s.route.ApplicationID)
	return s
}

// start loop to receive messages from route
func (s *sub) start() *sub {
	go s.loop()
	return s
}

// loop that will run in a goroutine and pipe messages from route to gcm
// Attention: in order for this loop to finish the route channel must be closed
func (s *sub) loop() {
	s.logger.Debug("Starting subscription loop")

	s.gcm.wg.Add(1)
	defer func() {
		s.logger.Debug("Subscription loop ended")
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

			s.logger.WithField("err", err).Error("Error pipelining message")
		}
	}
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
func (s *sub) store() {
	s.gcm.kvStore.Put(schema, s.route.ApplicationID, s.bytes())
}

func (s *sub) setLastID(ID uint64) {
	s.lastID = ID

	// update KV when last id is set
	s.store()
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
		s.setLastID(pm.message.ID)
		return s.handleGCMResponse(response)
	case err := <-pm.errC:
		s.logger.WithField("err", err).Error("Error sending message to GCM")
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
			s.logger.WithField("json_error", errText).Error("Subscription is not registered")
		} else {
			s.logger.WithField("json_error", errText).Error("Unexpected error while sending to GCM")
		}
	}

	return nil
}

// replace subscription with cannoical id, creates a new subscription but alters the route to
// have the new ApplicationID
func (s *sub) replaceCanonical(newGCMID string) error {
	s.logger.WithField("new_gcm_id", newGCMID).Info("Replacing with canonicalID")

	// delete current route from kvstore
	s.remove()

	// reuse the route but change the ApplicationID
	route := s.route
	route.ApplicationID = newGCMID
	newS := newSub(s.gcm, route, s.lastID)

	newS.add().start()
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
