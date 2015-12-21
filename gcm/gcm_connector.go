package gcm

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"

	"github.com/julienschmidt/httprouter"

	"fmt"
	"net/http"
)

type GCMConnector struct {
	router  server.PubSubSource
	kvStore store.KVStore
	mux     http.Handler
	prefix  string
}

func NewGCMConnector(prefix string) *GCMConnector {
	mux := httprouter.New()

	gcm := &GCMConnector{mux: mux, prefix: prefix}

	p := removeTrailingSlash(prefix)
	mux.POST(p+"/register/:userid/:gcmid/*topic", gcm.Register)

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

func (gcm *GCMConnector) Register(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	topic := "/" + params.ByName(`topic`)
	guble.Info("new registration to gcm connector userid=%q, gcmid=%q: %q", params.ByName(`userid`), params.ByName(`gcmid`), topic)
	fmt.Fprintf(w, "registered: %v\n", topic)
}

func removeTrailingSlash(path string) string {
	if len(path) > 0 && path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}
