package gcm

import (
	"github.com/alexjlockwood/gcm"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"

	log "github.com/Sirupsen/logrus"

	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

const (
	// registrationsSchema is the default sqlite schema for GCM
	registrationsSchema = "gcm_registration"

	// sendRetries is the number of retries when sending a message
	sendRetries = 5

	// broadcastRetries is the number of retries when broadcasting a message
	broadcastRetries = 3

	subscribePrefixPath = "subscribe"
)

// GCMConnector is the structure for handling the communication with Google Cloud Messaging
type GCMConnector struct {
	Sender        *gcm.Sender
	router        server.Router
	kvStore       store.KVStore
	prefix        string
	routerC       chan *server.MessageForRoute
	stopC         chan bool
	nWorkers      int
	wg            sync.WaitGroup
	broadcastPath string
}

// NewGCMConnector creates a new GCMConnector without starting it
func NewGCMConnector(router server.Router, prefix string, gcmAPIKey string, nWorkers int) (*GCMConnector, error) {

	kvStore, err := router.KVStore()
	if err != nil {
		return nil, err
	}

	conn := &GCMConnector{
		Sender:        &gcm.Sender{ApiKey: gcmAPIKey},
		router:        router,
		kvStore:       kvStore,
		prefix:        prefix,
		routerC:       make(chan *server.MessageForRoute, 1000*nWorkers),
		stopC:         make(chan bool, 1),
		nWorkers:      nWorkers,
		broadcastPath: removeTrailingSlash(prefix) + "/broadcast",
	}

	return conn, nil
}

// Start opens the connector, creates more goroutines / workers to handle messages coming from the router
func (conn *GCMConnector) Start() error {
	broadcastRoute := server.NewRoute(
		conn.broadcastPath,
		"gcm_connector",
		"gcm_connector",
		conn.routerC,
	)
	conn.router.Subscribe(broadcastRoute)
	go func() {
		//TODO Cosmin: should loadSubscriptions() be taken out of this goroutine, and executed before ?
		// (even if startup-time is longer, the routes are guaranteed to be there right after Start() returns)
		conn.loadSubscriptions()

		conn.wg.Add(conn.nWorkers)
		for id := 1; id <= conn.nWorkers; id++ {
			go conn.loopSendOrBroadcastMessage(id)
		}
	}()
	return nil
}

// Stop signals the closing of GCMConnector
func (conn *GCMConnector) Stop() error {
	log.WithFields(log.Fields{
		"module": "GCM",
	}).Debug("Stopping connector")

	close(conn.stopC)
	conn.wg.Wait()
	log.WithFields(log.Fields{
		"module": "GCM",
	}).Debug("Stopped connector")
	return nil
}

// Check returns nil if health-check succeeds, or an error if health-check fails
// by sending a request with only apikey. If the response is processed by the GCM endpoint
// the gcmStatus will be UP, otherwise the error from sending the message will be returned.
func (conn *GCMConnector) Check() error {
	payload := conn.parseMessageToMap(&protocol.Message{Body: []byte(`{"registration_ids":["ABC"]}`)})
	_, err := conn.Sender.Send(gcm.NewMessage(payload, ""), sendRetries)
	if err != nil {

		log.WithFields(log.Fields{
			"module": "GCM",
			"err":    err,
		}).Error("Error sending ping message")

		return err
	}
	return nil
}

// loopSendOrBroadcastMessage awaits in a loop for messages from router to be forwarded to GCM,
// until the stop-channel is closed
func (conn *GCMConnector) loopSendOrBroadcastMessage(id int) {
	defer conn.wg.Done()

	log.WithFields(log.Fields{
		"module": "GCM",
		"id":     id,
	}).Debug("Starting worker with ")

	for {
		select {
		case msg, opened := <-conn.routerC:
			if opened {
				if string(msg.Message.Path) == conn.broadcastPath {
					go conn.broadcastMessage(msg)
				} else {
					go conn.sendMessage(msg)
				}
			}
		case <-conn.stopC:

			log.WithFields(log.Fields{
				"module": "GCM",
				"id":     id,
			}).Debug("Stopping worker with")
			return
		}
	}
}

func (conn *GCMConnector) sendMessage(msg *server.MessageForRoute) {
	gcmID := msg.Route.ApplicationID

	payload := conn.parseMessageToMap(msg.Message)

	var messageToGcm = gcm.NewMessage(payload, gcmID)

	log.WithFields(log.Fields{
		"module":         "GCM",
		"gcmID":          gcmID,
		"channel_length": len(conn.routerC),
	}).Debug("Sending message to ")

	result, err := conn.Sender.Send(messageToGcm, sendRetries)
	if err != nil {
		log.WithFields(log.Fields{
			"module": "GCM",
			"gcmID":  gcmID,
			"err":    err,
		}).Error("Error sending message to")

		return
	}

	errorJSON := result.Results[0].Error
	if errorJSON != "" {
		conn.handleJSONError(errorJSON, gcmID, msg.Route)
	} else {
		log.WithFields(log.Fields{
			"module":    "GCM",
			"gcmID":     gcmID,
			"errorJSON": errorJSON,
		}).Debug("Delivered message to ")
	}

	// we only send to one receiver,
	// so we know that we can replace the old id with the first registration id (=canonical id)
	if result.CanonicalIDs != 0 {
		conn.replaceSubscriptionWithCanonicalID(msg.Route, result.Results[0].RegistrationID)
	}
}

func (conn *GCMConnector) broadcastMessage(msg *server.MessageForRoute) {
	topic := msg.Message.Path
	payload := conn.parseMessageToMap(msg.Message)
	log.WithFields(log.Fields{
		"module":         "GCM",
		"topic":          string(topic),
		"channel_length": len(conn.routerC),
	}).Debug("Broadcasting message with ")

	subscriptions := conn.kvStore.Iterate(registrationsSchema, "")
	count := 0
	for {
		select {
		case entry, ok := <-subscriptions:
			if !ok {
				log.WithFields(log.Fields{
					"module":          "GCM",
					"receivers_count": count,
				}).Info("Sent to message to ")
				return
			}
			gcmID := entry[0]
			//TODO collect 1000 gcmIds and send them in one request!
			broadcastMessage := gcm.NewMessage(payload, gcmID)
			go func() {
				//TODO error handling of response!
				_, err := conn.Sender.Send(broadcastMessage, broadcastRetries)
				log.WithFields(log.Fields{
					"module": "GCM",
					"gcmID":  gcmID,
				}).Debug("Sent broadcast message to")
				if err != nil {

					log.WithFields(log.Fields{
						"module": "GCM",
						"gmcId":  gcmID,
						"err":    err,
					}).Error("Error sending broadcast message to")
				}
			}()
			count++
		}
	}
}

func (conn *GCMConnector) parseMessageToMap(msg *protocol.Message) map[string]interface{} {
	payload := map[string]interface{}{}
	if msg.Body[0] == '{' {
		json.Unmarshal(msg.Body, &payload)
	} else {
		payload["message"] = msg.BodyAsString()
	}
	log.WithFields(log.Fields{
		"module":  "GCM",
		"payload": payload,
	}).Debug("Parsed message is:")
	return payload
}

func (conn *GCMConnector) replaceSubscriptionWithCanonicalID(route *server.Route, newGcmID string) {
	oldGcmID := route.ApplicationID
	topic := string(route.Path)
	userID := route.UserID

	log.WithFields(log.Fields{
		"module":      "GCM",
		"oldGcmId":    oldGcmID,
		"with":        "",
		"canonicalID": newGcmID,
	}).Info("Replacing ")

	conn.removeSubscription(route, oldGcmID)
	conn.subscribe(topic, userID, newGcmID)
}

func (conn *GCMConnector) handleJSONError(jsonError string, gcmID string, route *server.Route) {
	if jsonError == "NotRegistered" {

		log.WithFields(log.Fields{
			"module": "GCM",
			"gmcId":  gcmID,
		}).Debug("Removing not registered GCM registration for ")

		conn.removeSubscription(route, gcmID)
	} else if jsonError == "InvalidRegistration" {

		log.WithFields(log.Fields{
			"module": "GCM",
			"gmcId":  gcmID,
			"err":    "InvalidRegistration",
		}).Error("Not registered:")

	} else {
		log.WithFields(log.Fields{
			"module": "GCM",
			"gmcId":  gcmID,
			"err":    jsonError,
		}).Error("unexpected error while sending to GCM ")
	}
}

// GetPrefix is used to satisfy the HTTP handler interface
func (conn *GCMConnector) GetPrefix() string {
	return conn.prefix
}

func (conn *GCMConnector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {

		log.WithFields(log.Fields{
			"module": "GCM",
			"method": r.Method,
		}).Error("Only HTTP POST METHOD SUPPORTED but received type ")

		http.Error(w, "Permission Denied", http.StatusMethodNotAllowed)
		return
	}

	userID, gcmID, topic, err := conn.parseParams(r.URL.Path)
	if err != nil {

		log.WithFields(log.Fields{
			"module": "GCM",
			"method": r.Method,
			"err":    http.StatusBadRequest,
		}).Error("Invalid Parameters in request")

		http.Error(w, "Invalid Parameters in request", http.StatusBadRequest)
		return
	}
	conn.subscribe(topic, userID, gcmID)

	fmt.Fprintf(w, "registered: %v\n", topic)
}

// parseParams will parse the HTTP URL with format /gcm/:userid/:gcmid/subscribe/*topic
// returning the parsed Params, or error if the request is not in the correct format
func (conn *GCMConnector) parseParams(path string) (userID, gcmID, topic string, err error) {
	currentURLPath := removeTrailingSlash(path)

	if strings.HasPrefix(currentURLPath, conn.prefix) != true {
		err = errors.New("gcm: GCM request is not starting with gcm prefix")
		return
	}
	pathAfterPrefix := strings.TrimPrefix(currentURLPath, conn.prefix)

	splitParams := strings.SplitN(pathAfterPrefix, "/", 3)
	if len(splitParams) != 3 {
		err = errors.New("gcm: GCM request has wrong number of params")
		return
	}
	userID = splitParams[0]
	gcmID = splitParams[1]

	if strings.HasPrefix(splitParams[2], subscribePrefixPath+"/") != true {
		err = errors.New("gcm: GCM request third param is not subscribe")
		return
	}
	topic = strings.TrimPrefix(splitParams[2], subscribePrefixPath)
	return userID, gcmID, topic, nil
}

func (conn *GCMConnector) subscribe(topic string, userID string, gcmID string) {

	log.WithFields(log.Fields{
		"module": "GCM",
		"userID": userID,
		"gcmId":  gcmID,
		"topic":  string(topic),
	}).Info("GCM connector registration to")

	route := server.NewRoute(topic, gcmID, userID, conn.routerC)

	conn.router.Subscribe(route)
	conn.saveSubscription(userID, topic, gcmID)
}

func (conn *GCMConnector) removeSubscription(route *server.Route, gcmID string) {
	conn.router.Unsubscribe(route)
	conn.kvStore.Delete(registrationsSchema, gcmID)
}

func (conn *GCMConnector) saveSubscription(userID, topic, gcmID string) {
	conn.kvStore.Put(registrationsSchema, gcmID, []byte(userID+":"+topic))
}

func (conn *GCMConnector) loadSubscriptions() {
	subscriptions := conn.kvStore.Iterate(registrationsSchema, "")
	count := 0
	for {
		select {
		case entry, ok := <-subscriptions:
			if !ok {
				log.WithFields(log.Fields{
					"module":                 "GCM",
					"gcm_subscription_count": count,
				}).Info("Renewed ")
				return
			}
			gcmID := entry[0]
			splitValue := strings.SplitN(entry[1], ":", 2)
			userID := splitValue[0]
			topic := splitValue[1]

			log.WithFields(log.Fields{
				"module": "GCM",
				"userID": userID,
				"gcmId":  gcmID,
				"topic":  string(topic),
			}).Debug("renewing GCM subscription")

			route := server.NewRoute(topic, gcmID, userID, conn.routerC)
			conn.router.Subscribe(route)
			count++
		}
	}
}

func removeTrailingSlash(path string) string {
	if len(path) > 1 && path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}
