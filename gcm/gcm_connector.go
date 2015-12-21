package gcm

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"

	"github.com/julienschmidt/httprouter"

	"fmt"
	"net/http"
)

const GCM_REGISTRATIONS_SCHEMA = "gcm_registration"

type GCMConnector struct {
	router             server.PubSubSource
	kvStore            store.KVStore
	mux                http.Handler
	prefix             string
	channelFromRouter  chan *guble.Message
	closeRouteByRouter chan string
}

func NewGCMConnector(prefix string) *GCMConnector {
	mux := httprouter.New()

	channelFromRouter := make(chan *guble.Message, 1000)
	closeRouteByRouter := make(chan string)
	gcm := &GCMConnector{mux: mux, prefix: prefix, channelFromRouter: channelFromRouter, closeRouteByRouter: closeRouteByRouter}

	p := removeTrailingSlash(prefix)
	mux.POST(p+"/:userid/:gcmid/subscribe/*topic", gcm.Subscribe)

	return gcm
}

func (gcm *GCMConnector) GetPrefix() string {
	return gcm.prefix
}

func (gcm *GCMConnector) SetRouter(router server.PubSubSource) {
	gcm.router = router
}

func (gcm *GCMConnector) SetKVStore(kvStore server.KVStore) {
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
	gcm.router.Subscribe(route)

	gcm.saveSubscription(userid, gcmid, topic)

	fmt.Fprintf(w, "registered: %v\n", topic)
}

func (gcm *GCMConnector) saveSubscription(userid, gcmid, topic string) {
	gcm.kvStore.Put(GCM_REGISTRATIONS_SCHEMA, userid+":"+topic, []byte(gcmid))
}

func removeTrailingSlash(path string) string {
	if len(path) > 0 && path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}
