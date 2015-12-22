package gcm

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"

	"github.com/julienschmidt/httprouter"

	"github.com/alexjlockwood/gcm"

	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const GCM_REGISTRATIONS_SCHEMA = "gcm_registration"

type GCMConnector struct {
	router             server.PubSubSource
	kvStore            store.KVStore
	mux                http.Handler
	prefix             string
	channelFromRouter  chan server.MsgAndRoute
	closeRouteByRouter chan string
	stopChan           chan bool
	sender             *gcm.Sender
}

func NewGCMConnector(prefix string, gcmApiKey string) *GCMConnector {
	mux := httprouter.New()
	gcm := &GCMConnector{
		mux:                mux,
		prefix:             prefix,
		channelFromRouter:  make(chan server.MsgAndRoute, 1000),
		closeRouteByRouter: make(chan string),
		stopChan:           make(chan bool, 1),
		sender:             &gcm.Sender{ApiKey: gcmApiKey},
	}

	mux.POST(removeTrailingSlash(gcm.prefix)+"/:userid/:gcmid/subscribe/*topic", gcm.Subscribe)

	return gcm
}

func (gcmConnector *GCMConnector) Start() {
	broadcastRoute := server.NewRoute(removeTrailingSlash(gcmConnector.prefix)+"/broadcast", gcmConnector.channelFromRouter, gcmConnector.closeRouteByRouter, "gcm_connector", "gcm_connector")
	gcmConnector.router.Subscribe(broadcastRoute)
	go func() {
		gcmConnector.loadSubscriptions()

		for {
			select {
			case msg := <-gcmConnector.channelFromRouter:
				if string(msg.Message.Path) == gcmConnector.prefix+"/broadcast" {
					go gcmConnector.broadcastMessage(msg)
				} else {
					go gcmConnector.sendMessageToGCM(msg)
				}
			case <-gcmConnector.stopChan:
				return
			}
		}
	}()
}

func (gcmConnector *GCMConnector) sendMessageToGCM(msg server.MsgAndRoute) {
	gcmId := msg.Route.ApplicationId

	payload := gcmConnector.parseMessageToMap(msg.Message)

	var messageToGcm = gcm.NewMessage(payload, gcmId)
	guble.Info("sending message to %v ...", gcmId)
	result, err := gcmConnector.sender.Send(messageToGcm, 5)
	if err != nil {
		guble.Err("error sending message to cgm cgmid=%v: %v", gcmId, err.Error())
		return
	}

	errorJson := result.Results[0].Error
	if errorJson != "" {
		gcmConnector.handleJsonError(errorJson, gcmId, msg.Route)
	} else {
		guble.Debug("delivered message to gcm cgmid=%v: %v", gcmId, errorJson)
	}

	//we only send to one receiver, so we know that we can replace the old id with the first registration id (=canonical id)
	if result.CanonicalIDs != 0 {
		gcmConnector.replaceSubscriptionWithCanonicalID(msg.Route, result.Results[0].RegistrationID)
	}
}

func (gcmConnector *GCMConnector) parseMessageToMap(msg *guble.Message) map[string]interface{} {
	payload := map[string]interface{}{}
	if msg.Body[0] == '{' {
		json.Unmarshal(msg.Body, &payload)
	} else {
		payload["message"] = msg.BodyAsString()
	}
	return payload
}

func (gcmConnector *GCMConnector) broadcastMessage(msg server.MsgAndRoute) {
	//TODO

	/*
		payload := map[string]interface{}{"message": msg.Message.BodyAsString()}

		var messageToGcm = gcm.NewMessage(payload, gcmId)
		guble.Info("sending message to %v ...", gcmId)
		result, err := gcmConnector.sender.Send(messageToGcm, 5)
		if err != nil {
			guble.Err("error sending message to cgm cgmid=%v: %v", gcmId, err.Error())
			return
		}
	*/
}

func (gcmConnector *GCMConnector) replaceSubscriptionWithCanonicalID(route *server.Route, newId string) {
	oldGcmId := route.ApplicationId
	topic := string(route.Path)
	userId := route.UserId

	guble.Info("replacing old gcmId %v with canonicalId %v", oldGcmId, newId)
	gcmConnector.removeSubscription(route)
	gcmConnector.subscribe(topic, userId, newId)
}

func (gcmConnector *GCMConnector) handleJsonError(jsonError string, gcmId string, route *server.Route) {
	if jsonError == "NotRegistered" {
		guble.Debug("remove not registered cgm registration cgmid=%v", gcmId)
		gcmConnector.removeSubscription(route)
	} else if jsonError == "InvalidRegistration" {
		guble.Err("the cgmid=%v is not registered. %v", gcmId, jsonError)
	} else {
		guble.Err("unexpected error while sending to cgm cgmid=%v: %v", gcmId, jsonError)
	}
}

func (gcmConnector *GCMConnector) Stop() error {
	gcmConnector.stopChan <- true
	return nil
}

func (gcmConnector *GCMConnector) GetPrefix() string {
	return gcmConnector.prefix
}

func (gcmConnector *GCMConnector) SetRouter(router server.PubSubSource) {
	gcmConnector.router = router
}

func (gcmConnector *GCMConnector) SetKVStore(kvStore store.KVStore) {
	gcmConnector.kvStore = kvStore
}

func (gcmConnector *GCMConnector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	gcmConnector.mux.ServeHTTP(w, r)
}

func (gcmConnector *GCMConnector) Subscribe(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	topic := params.ByName(`topic`)
	userid := params.ByName("userid")
	gcmid := params.ByName(`gcmid`)

	gcmConnector.subscribe(topic, userid, gcmid)

	fmt.Fprintf(w, "registered: %v\n", topic)
}

func (gcmConnector *GCMConnector) subscribe(topic string, userid string, gcmid string) {
	guble.Info("gcm connector registration to userid=%q, gcmid=%q: %q", userid, gcmid, topic)

	route := server.NewRoute(topic, gcmConnector.channelFromRouter, gcmConnector.closeRouteByRouter, gcmid, userid)
	route.Id = "gcm-" + gcmid

	gcmConnector.router.Subscribe(route)
	gcmConnector.saveSubscription(userid, topic, gcmid)
}

func (gcmConnector *GCMConnector) removeSubscription(route *server.Route) {
	gcmConnector.router.Unsubscribe(route)
	gcmConnector.kvStore.Delete(GCM_REGISTRATIONS_SCHEMA, route.UserId+":"+string(route.Path))
}

func (gcmConnector *GCMConnector) saveSubscription(userid, topic, gcmid string) {
	gcmConnector.kvStore.Put(GCM_REGISTRATIONS_SCHEMA, userid+":"+topic, []byte(gcmid))
}

func (gcmConnector *GCMConnector) loadSubscriptions() {
	subscriptions := gcmConnector.kvStore.Iterate(GCM_REGISTRATIONS_SCHEMA, "")
	count := 0
	for {
		select {
		case entry, ok := <-subscriptions:
			if !ok {
				guble.Info("renewed %v gcm subscriptions", count)
				return
			}
			splitedKey := strings.SplitN(entry[0], ":", 2)
			userid := splitedKey[0]
			topic := splitedKey[1]
			gcmid := entry[1]

			guble.Debug("renew gcm subscription: user=%v, topic=%v, gcmid=%v", userid, topic, gcmid)
			route := server.NewRoute(topic, gcmConnector.channelFromRouter, gcmConnector.closeRouteByRouter, gcmid, userid)
			gcmConnector.router.Subscribe(route)
			count++
		}
	}
}

func removeTrailingSlash(path string) string {
	if len(path) > 0 && path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}
