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
	closeRouteByRouter chan server.Route
	stopChan           chan bool
	sender             *gcm.Sender
}

func NewGCMConnector(prefix string, gcmApiKey string) *GCMConnector {
	mux := httprouter.New()
	gcm := &GCMConnector{
		mux:               mux,
		prefix:            prefix,
		channelFromRouter: make(chan server.MsgAndRoute, 1000),
		stopChan:          make(chan bool, 1),
		sender:            &gcm.Sender{ApiKey: gcmApiKey},
	}

	mux.POST(removeTrailingSlash(gcm.prefix)+"/:userid/:gcmid/subscribe/*topic", gcm.Subscribe)

	return gcm
}

func (gcmConnector *GCMConnector) Start() error {
	broadcastRoute := server.NewRoute(removeTrailingSlash(gcmConnector.prefix)+"/broadcast", gcmConnector.channelFromRouter, "gcm_connector", "gcm_connector")
	gcmConnector.router.Subscribe(broadcastRoute)
	go func() {
		gcmConnector.loadSubscriptions()

		for {
			select {
			case msg := <-gcmConnector.channelFromRouter:
				if string(msg.Message.Path) == removeTrailingSlash(gcmConnector.prefix)+"/broadcast" {
					go gcmConnector.broadcastMessage(msg)
				} else {
					go gcmConnector.sendMessageToGCM(msg)
				}
			case <-gcmConnector.stopChan:
				return
			}
		}
	}()
	return nil
}

func (gcmConnector *GCMConnector) sendMessageToGCM(msg server.MsgAndRoute) {
	gcmId := msg.Route.ApplicationId

	payload := gcmConnector.parseMessageToMap(msg.Message)

	var messageToGcm = gcm.NewMessage(payload, gcmId)
	guble.Info("sending message to %v ...", gcmId)
	result, err := gcmConnector.sender.Send(messageToGcm, 5)
	if err != nil {
		guble.Err("error sending message to cgmid=%v: %v", gcmId, err.Error())
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
	guble.Debug("parsed message is: %v", payload)
	return payload
}

func (gcmConnector *GCMConnector) broadcastMessage(msg server.MsgAndRoute) {
	topic := msg.Message.Path
	payload := gcmConnector.parseMessageToMap(msg.Message)
	guble.Info("broadcasting message with topic %v ...", string(topic))

	subscriptions := gcmConnector.kvStore.Iterate(GCM_REGISTRATIONS_SCHEMA, "")
	count := 0
	for {
		select {
		case entry, ok := <-subscriptions:
			if !ok {
				guble.Info("send message to %v receivers", count)
				return
			}
			gcmId := entry[0]
			//TODO collect 1000 gcmIds and send them in one request!
			broadcastMessage := gcm.NewMessage(payload, gcmId)
			go func() {
				//TODO error handling of response!
				_, err := gcmConnector.sender.Send(broadcastMessage, 3)
				guble.Debug("sent broadcast message to gcmId=%v", gcmId)
				if err != nil {
					guble.Err("error sending broadcast message to cgmid=%v: %v", gcmId, err.Error())
				}
			}()
			count++
		}
	}
}

func (gcmConnector *GCMConnector) replaceSubscriptionWithCanonicalID(route *server.Route, newGcmId string) {
	oldGcmId := route.ApplicationId
	topic := string(route.Path)
	userId := route.UserId

	guble.Info("replacing old gcmId %v with canonicalId %v", oldGcmId, newGcmId)
	gcmConnector.removeSubscription(route, oldGcmId)
	gcmConnector.subscribe(topic, userId, newGcmId)
}

func (gcmConnector *GCMConnector) handleJsonError(jsonError string, gcmId string, route *server.Route) {
	if jsonError == "NotRegistered" {
		guble.Debug("remove not registered cgm registration cgmid=%v", gcmId)
		gcmConnector.removeSubscription(route, gcmId)
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

	route := server.NewRoute(topic, gcmConnector.channelFromRouter, gcmid, userid)

	gcmConnector.router.Subscribe(route)
	gcmConnector.saveSubscription(userid, topic, gcmid)
}

func (gcmConnector *GCMConnector) removeSubscription(route *server.Route, gcmId string) {
	gcmConnector.router.Unsubscribe(route)
	gcmConnector.kvStore.Delete(GCM_REGISTRATIONS_SCHEMA, gcmId)
}

func (gcmConnector *GCMConnector) saveSubscription(userid, topic, gcmid string) {
	gcmConnector.kvStore.Put(GCM_REGISTRATIONS_SCHEMA, gcmid, []byte(userid+":"+topic))
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
			gcmId := entry[0]
			splitedValue := strings.SplitN(entry[1], ":", 2)
			userid := splitedValue[0]
			topic := splitedValue[1]

			guble.Debug("renew gcm subscription: user=%v, topic=%v, gcmid=%v", userid, topic, gcmId)
			route := server.NewRoute(topic, gcmConnector.channelFromRouter, gcmId, userid)
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
