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

func createSub(gcm *GCMConnector, route *server.Route, lastID uint64) *sub {
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

func newSub(gcm *GCMConnector, topic, userID, gcmID string, lastID uint64) *sub {
	route := server.NewRoute(topic, gcmID, userID, subBufferSize)
	return createSub(gcm, route, 0).add().start()
}

// sub represent a GCM subscription
type sub struct {
	gcm    *GCMConnector
	route  *server.Route
	lastID uint64 // Last sent message id

	logger *log.Entry
}

// save subscription in KV
// delete subscription from KV
// loop (receive messages from router and send them to gcm pipeline)
// replace subscription with cannoical id (this remains in the gcm as it will remove the subscription)

// subscribe to router and add the subscription to GCM subs list
func (s *sub) add() *sub {
	s.logger.Debug("Subscribed")

	s.gcm.router.Subscribe(s.route)
	s.store()
	return s
}

// unsubscribe from router and remove from GCM subs list
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
func (s *sub) loop() {
	s.gcm.wg.Add(1)
	defer func() { s.gcm.wg.Done() }()

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
	if err := s.jsonError(response); err != nil {
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

func (s *sub) jsonError(response *gcm.Response) error {
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

func (s *sub) replaceCanonical(newGCMID string) error {
	s.logger.WithField("new_gcm_id", newGCMID).Info("Replacing with canonicalID")

	// delete current route from kvstore
	s.remove()

	// reuse the route but change the ApplicationID
	route := s.route
	route.ApplicationID = newGCMID
	newS := createSub(s.gcm, route, s.lastID)
	newS.add().start()
	return errSubReplaced
}

func newPipeMessage(s *sub, m *protocol.Message) *pipeMessage {
	return &pipeMessage{s, m, make(chan *gcm.Response), make(chan error)}
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
