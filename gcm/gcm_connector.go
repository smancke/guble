package gcm

import (
	"github.com/alexjlockwood/gcm"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"

	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// GCM_REGISTRATIONS_SCHEMA is the default sqlite schema for gcm
const GCM_REGISTRATIONS_SCHEMA = "gcm_registration"

// GCMConnector is the structure for handling the communication with Google Cloud Messaging
type GCMConnector struct {
	router             server.Router
	kvStore            store.KVStore
	prefix             string
	channelFromRouter  chan server.MsgAndRoute
	closeRouteByRouter chan server.Route
	stopChan           chan bool
	sender             *gcm.Sender
}

// NewGCMConnector creates a new gcmConnector without starting it
func NewGCMConnector(router server.Router, prefix string, gcmAPIKey string) (*GCMConnector, error) {

	kvStore, err := router.KVStore()
	if err != nil {
		return nil, err
	}

	gcm := &GCMConnector{
		router:            router,
		kvStore:           kvStore,
		prefix:            prefix,
		channelFromRouter: make(chan server.MsgAndRoute, 1000),
		stopChan:          make(chan bool, 1),
		sender:            &gcm.Sender{ApiKey: gcmAPIKey},
	}

	return gcm, nil
}

// Start opens the connector and awaits for messages from router to be forwarded to gcm until the stop signal is emitted
func (conn *GCMConnector) Start() error {
	broadcastRoute := server.NewRoute(removeTrailingSlash(conn.prefix)+"/broadcast", conn.channelFromRouter, "gcm_connector", "gcm_connector")
	conn.router.Subscribe(broadcastRoute)
	go func() {
		conn.loadSubscriptions()

		for {
			select {
			case msg := <-conn.channelFromRouter:
				if string(msg.Message.Path) == removeTrailingSlash(conn.prefix)+"/broadcast" {
					go conn.broadcastMessage(msg)
				} else {
					go conn.sendMessage(msg)
				}
			case <-conn.stopChan:
				return
			}
		}
	}()
	return nil
}

func (conn *GCMConnector) Check() error {
	return nil
}

func (conn *GCMConnector) sendMessage(msg server.MsgAndRoute) {
	gcmID := msg.Route.ApplicationID

	payload := conn.parseMessageToMap(msg.Message)

	var messageToGcm = gcm.NewMessage(payload, gcmID)
	protocol.Info("sending message to %v ...", gcmID)
	result, err := conn.sender.Send(messageToGcm, 5)
	if err != nil {
		protocol.Err("error sending message to GCM gcmId=%v: %v", gcmID, err.Error())
		return
	}

	errorJSON := result.Results[0].Error
	if errorJSON != "" {
		conn.handleJSONError(errorJSON, gcmID, msg.Route)
	} else {
		protocol.Debug("delivered message to GCM gcmId=%v: %v", gcmID, errorJSON)
	}

	// we only send to one receiver,
	// so we know that we can replace the old id with the first registration id (=canonical id)
	if result.CanonicalIDs != 0 {
		conn.replaceSubscriptionWithCanonicalID(msg.Route, result.Results[0].RegistrationID)
	}
}

func (conn *GCMConnector) parseMessageToMap(msg *protocol.Message) map[string]interface{} {
	payload := map[string]interface{}{}
	if msg.Body[0] == '{' {
		json.Unmarshal(msg.Body, &payload)
	} else {
		payload["message"] = msg.BodyAsString()
	}
	protocol.Debug("parsed message is: %v", payload)
	return payload
}

func (conn *GCMConnector) broadcastMessage(msg server.MsgAndRoute) {
	topic := msg.Message.Path
	payload := conn.parseMessageToMap(msg.Message)
	protocol.Info("broadcasting message with topic %v ...", string(topic))

	subscriptions := conn.kvStore.Iterate(GCM_REGISTRATIONS_SCHEMA, "")
	count := 0
	for {
		select {
		case entry, ok := <-subscriptions:
			if !ok {
				protocol.Info("send message to %v receivers", count)
				return
			}
			gmcID := entry[0]
			//TODO collect 1000 gcmIds and send them in one request!
			broadcastMessage := gcm.NewMessage(payload, gmcID)
			go func() {
				//TODO error handling of response!
				_, err := conn.sender.Send(broadcastMessage, 3)
				protocol.Debug("sent broadcast message to gcmId=%v", gmcID)
				if err != nil {
					protocol.Err("error sending broadcast message to gcmId=%v: %v", gmcID, err.Error())
				}
			}()
			count++
		}
	}
}

func (conn *GCMConnector) replaceSubscriptionWithCanonicalID(route *server.Route, newGcmID string) {
	oldGcmID := route.ApplicationID
	topic := string(route.Path)
	userID := route.UserID

	protocol.Info("replacing old gcmId %v with canonicalId %v", oldGcmID, newGcmID)

	conn.removeSubscription(route, oldGcmID)
	conn.subscribe(topic, userID, newGcmID)
}

func (conn *GCMConnector) handleJSONError(jsonError string, gcmID string, route *server.Route) {
	if jsonError == "NotRegistered" {
		protocol.Debug("remove not registered cgm registration gcmid=%v", gcmID)
		conn.removeSubscription(route, gcmID)
	} else if jsonError == "InvalidRegistration" {
		protocol.Err("the cgmid=%v is not registered. %v", gcmID, jsonError)
	} else {
		protocol.Err("unexpected error while sending to cgm gcmId=%v: %v", gcmID, jsonError)
	}
}

// Stop signals the closing of gcmConnector
func (conn *GCMConnector) Stop() error {
	conn.stopChan <- true
	return nil
}

// GetPrefix is used to satisfy the HTTP handler interface
func (conn *GCMConnector) GetPrefix() string {
	return conn.prefix
}

func (conn *GCMConnector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		protocol.Err("Only HTTP POST METHOD SUPPORTED but received type=[%s]", r.Method)
		http.Error(w, "Permission Denied", http.StatusMethodNotAllowed)
		return
	}

	userID, gcmID, topic, err := conn.parseParams(r.URL.Path)
	if err != nil {
		http.Error(w, "Invalid Parameters in request", http.StatusBadRequest)
		return
	}
	conn.subscribe(topic, userID, gcmID)

	fmt.Fprintf(w, "registered: %v\n", topic)
}

// parseParams will parse the HTTP URL with format /gcm/:userid/:gcmid/subscribe/*topic
// returning the parsed Params, or error if the request is not in the correct format
func (conn *GCMConnector) parseParams(path string) (userID, gcmID, topic string, err error) {
	subscribePrefixPath := "subscribe"
	currentURLPath := removeTrailingSlash(path)

	if strings.HasPrefix(currentURLPath, conn.prefix) != true {
		err = errors.New("Gcm request is not starting with gcm prefix")
		return
	}
	pathAfterPrefix := strings.TrimPrefix(currentURLPath, conn.prefix)

	splitParams := strings.SplitN(pathAfterPrefix, "/", 3)
	if len(splitParams) != 3 {
		err = errors.New("Gcm request has wrong number of params")
		return
	}
	userID = splitParams[0]
	gcmID = splitParams[1]

	if strings.HasPrefix(splitParams[2], subscribePrefixPath+"/") != true {
		err = errors.New("Gcm request third param is not subscribe")
		return
	}
	topic = strings.TrimPrefix(splitParams[2], subscribePrefixPath)
	return userID, gcmID, topic, nil
}

func (conn *GCMConnector) subscribe(topic string, userID string, gcmID string) {
	protocol.Info("gcm connector registration to userid=%q, gcmid=%q: %q", userID, gcmID, topic)

	route := server.NewRoute(topic, conn.channelFromRouter, gcmID, userID)

	conn.router.Subscribe(route)
	conn.saveSubscription(userID, topic, gcmID)
}

func (conn *GCMConnector) removeSubscription(route *server.Route, gcmID string) {
	conn.router.Unsubscribe(route)
	conn.kvStore.Delete(GCM_REGISTRATIONS_SCHEMA, gcmID)
}

func (conn *GCMConnector) saveSubscription(userID, topic, gcmID string) {
	conn.kvStore.Put(GCM_REGISTRATIONS_SCHEMA, gcmID, []byte(userID+":"+topic))
}

func (conn *GCMConnector) loadSubscriptions() {
	subscriptions := conn.kvStore.Iterate(GCM_REGISTRATIONS_SCHEMA, "")
	count := 0
	for {
		select {
		case entry, ok := <-subscriptions:
			if !ok {
				protocol.Info("renewed %v gcm subscriptions", count)
				return
			}
			gcmID := entry[0]
			splitValue := strings.SplitN(entry[1], ":", 2)
			userID := splitValue[0]
			topic := splitValue[1]

			protocol.Debug("renew gcm subscription: user=%v, topic=%v, gcmid=%v", userID, topic, gcmID)
			route := server.NewRoute(topic, conn.channelFromRouter, gcmID, userID)
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
