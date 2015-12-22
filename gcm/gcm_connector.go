package gcm

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"

	"github.com/julienschmidt/httprouter"

	"github.com/alexjlockwood/gcm"

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

	p := removeTrailingSlash(prefix)
	mux.POST(p+"/:userid/:gcmid/subscribe/*topic", gcm.Subscribe)

	return gcm
}

func (gcmConnector *GCMConnector) Start() {
	go func() {
		gcmConnector.loadSubscriptions()

		for {
			select {
			case msg := <-gcmConnector.channelFromRouter:
				var gcmId = msg.Route.ApplicationId
				payload := map[string]interface{}{"message": msg.Message.BodyAsString()}
				var messageToGcm = gcm.NewMessage(payload, gcmId)
				guble.Info("sending message to %v ...", gcmId)
				gcmConnector.sender.Send(messageToGcm, 1)
			case <-gcmConnector.stopChan:
				return
			}
		}
	}()
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

	guble.Info("gcm connector registration to userid=%q, gcmid=%q: %q", userid, gcmid, topic)

	route := server.NewRoute(topic, gcmConnector.channelFromRouter, gcmConnector.closeRouteByRouter, gcmid, userid)
	route.Id = "gcm-" + gcmid

	gcmConnector.router.Subscribe(route)

	gcmConnector.saveSubscription(userid, topic, gcmid)

	fmt.Fprintf(w, "registered: %v\n", topic)
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
