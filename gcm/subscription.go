package gcm

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/alexjlockwood/gcm"
	"github.com/smancke/guble/server"
	"strings"
)

const (
	// default subscription channel buffer size
	subBufferSize = 50
)

func newSub(gcm *GCMConnector, topic, userID, gcmID string) *sub {
	return &sub{
		gcm:    gcm,
		router: gcm.router,
		route:  server.NewRoute(topic, gcmID, userID, subBufferSize),

		logger: logger.WithField("gcmID", s.route.ApplicationID),
	}.add().start()
}

// sub represent a GCM subscription
type sub struct {
	gcm    *GCMConnector
	router *server.Router
	route  *server.Route
	lastID int // Last sent message id

	logger *log.Entry
}

// save subscription in KV
// delete subscription from KV
// loop (receive messages from router and send them to gcm pipeline)
// replace subscription with cannoical id (this remains in the gcm as it will remove the subscription)

// subscribe to router and add the subscription to GCM subs list
func (s *sub) add() *sub {
	s.logger.WithFields(log.Fields{
		"user_id": s.route.userID,
		"gcm_id":  s.route.ApplicationID,
		"topic":   s.route.Path,
	}, "Subscribed")

	s.router.Subscribe(s.route)
	s.store()
}

// unsubscribe from router and remove from GCM subs list
func (s *sub) remove() *sub {
	s.router.Unsubscribe(s.route)
	s.gcm.kvStore.Delete(schema, s.route.ApplicationID)
}

// start loop to receive messages from route
func (s *sub) start() *sub {
	go s.loop()
	return s
}

// loop that will run in a goroutine and pipe messages from route to gcm
func (s *sub) loop() {
	for {
		select {
		case m, opened := <-s.route.MessagesChannel():
			if err := s.pipe(m); err != nil {
				s.logger.WithField("err", err).Error("Error pipelining message")
			}
		case <-s.gcm.stopC:
			return
		}
	}
}

// return bytes data to store in kvStore
func (s *sub) bytes() []byte {
	return []byte(strings.Join([]string{
		s.route.UserID,
		string(s.route.Path),
		s.lastID,
	}, ":"))
}

// store data in kvstore
func (s *sub) store() {
	s.gcm.kvStore.Put(schema, gcmID, s.bytes())
}

// sends a message into the pipeline and waits for response saving the last id
// in the kvstore
func (s *sub) pipe(m *protocol.Message) error {
	s.logger.WithFields(log.Fields{
		"gcmID": gcmID,
		"err":   err,
	}).Error("Error sending message to GCM")

	// errorJSON := result.Results[0].Error
	// if errorJSON != "" {
	// 	conn.handleJSONError(errorJSON, gcmID, m.Route)
	// } else {
	// 	protocol.Debug("gcm: delivered message to GCM gcmID=%v: %v", gcmID, errorJSON)
	// }

	// // we only send to one receiver,
	// // so we know that we can replace the old id with the first registration id (=canonical id)
	// if result.CanonicalIDs != 0 {
	// 	conn.replaceSubscriptionWithCanonicalID(m.Route, result.Results[0].RegistrationID)
	// }
	return nil
}

func (s *sub) handleJSONError(jsonError string) {
	if jsonError == "NotRegistered" {
		s.logger().
			protocol.Debug("gcm: removing not registered GCM registration gcmID=%v", gcmID)
		sub.dele(route, gcmID)
	} else if jsonError == "InvalidRegistration" {
		protocol.Err("gcm: the gcmID=%v is not registered. %v", gcmID, jsonError)
	} else {
		protocol.Err("gcm: unexpected error while sending to GCM gcmID=%v: %v", gcmID, jsonError)
	}
}

// func (conn *GCMConnector) replaceSubscriptionWithCanonicalID(route *server.Route, newGcmID string) {
// 	oldGcmID := route.ApplicationID
// 	topic := string(route.Path)
// 	userID := route.UserID

// 	protocol.Info("gcm: replacing old gcmID %v with canonicalId %v", oldGcmID, newGcmID)

// 	conn.removeSubscription(route, oldGcmID)
// 	conn.subscribe(topic, userID, newGcmID)
// }

// Pipeline message
type pipeMessage struct {
	*sub
	message *Message
	resultC chan gcm.Result
	errC    chan error
}

func (pm *pipeMessage) payload() map[string]interface{} {
	payload := make(map[string]interface{})

	if pm.message.Body[0] == '{' {
		json.Unmarshal(pm.message.Body, &payload)
	} else {
		payload["message"] = msg.BodyAsString()
	}

	pm.logger.WithField("payload", payload).Debug("Parsed message")
	return payload

}
