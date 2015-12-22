package gcm

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"

	"github.com/julienschmidt/httprouter"

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
	channelFromRouter  chan *guble.Message
	closeRouteByRouter chan string
	stopChan           chan bool
}

func NewGCMConnector(prefix string) *GCMConnector {
	mux := httprouter.New()
	gcm := &GCMConnector{
		mux:                mux,
		prefix:             prefix,
		channelFromRouter:  make(chan *guble.Message, 1000),
		closeRouteByRouter: make(chan string),
		stopChan:           make(chan bool, 1),
	}

	p := removeTrailingSlash(prefix)
	mux.POST(p+"/:userid/:gcmid/subscribe/*topic", gcm.Subscribe)

	return gcm
}

func (gcm *GCMConnector) Start() {
	go func() {
		gcm.loadSubscriptions()

		for {
			select {
			case msg := <-gcm.channelFromRouter:
				guble.Err("!!!TODO: send message to gcm service %v,%v", msg.Id, msg.Path)
			case <-gcm.stopChan:
				return
			}

		}
	}()
}

func (gcm *GCMConnector) Stop() error {
	gcm.stopChan <- true
	return nil
}

func (gcm *GCMConnector) GetPrefix() string {
	return gcm.prefix
}

func (gcm *GCMConnector) SetRouter(router server.PubSubSource) {
	gcm.router = router
}

func (gcm *GCMConnector) SetKVStore(kvStore store.KVStore) {
	gcm.kvStore = kvStore
}

func (gcm *GCMConnector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	gcm.mux.ServeHTTP(w, r)
}

func (gcm *GCMConnector) Subscribe(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	topic := params.ByName(`topic`)
	userid := params.ByName("userid")
	gcmid := params.ByName(`gcmid`)

	guble.Info("gcm connector registration to userid=%q, gcmid=%q: %q", userid, gcmid, topic)

	route := server.NewRoute(topic, gcm.channelFromRouter, gcm.closeRouteByRouter, gcmid, userid)

	// TODO: check, that multiple equal subscriptions are handled only once
	gcm.router.Subscribe(route)

	gcm.saveSubscription(userid, topic, gcmid)

	fmt.Fprintf(w, "registered: %v\n", topic)
}

func (gcm *GCMConnector) saveSubscription(userid, topic, gcmid string) {
	gcm.kvStore.Put(GCM_REGISTRATIONS_SCHEMA, userid+":"+topic, []byte(gcmid))
}

func (gcm *GCMConnector) loadSubscriptions() {
	subscriptions := gcm.kvStore.Iterate(GCM_REGISTRATIONS_SCHEMA, "")
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
			route := server.NewRoute(topic, gcm.channelFromRouter, gcm.closeRouteByRouter, gcmid, userid)
			gcm.router.Subscribe(route)
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
