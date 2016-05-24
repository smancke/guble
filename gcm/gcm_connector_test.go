package gcm

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"

	"github.com/golang/mock/gomock"
//	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"

	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

var ctrl *gomock.Controller

func TestPostMessage(t *testing.T) {
	defer initCtrl(t)()

	a := assert.New(t)

	// given:  a rest api with a message sink
	routerMock := NewMockRouter(ctrl)

	kvStore := store.NewMemoryKVStore()
	routerMock.EXPECT().KVStore().Return(kvStore, nil)

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(route *server.Route) {
		a.Equal("/notifications", string(route.Path))
		a.Equal("marvin", route.UserID)
		a.Equal("gcmId123", route.ApplicationID)
	})

	gcm, err := NewGCMConnector(routerMock, "/gcm/", "testApi")
	a.Nil(err)

	url, _ := url.Parse("http://localhost/gcm/marvin/gcmId123/subscribe/notifications")
	// and a http context
	req := &http.Request{URL: url}
	w := httptest.NewRecorder()

	//params := httprouter.Params{
	//	httprouter.Param{Key: "userid", Value: "marvin"},
	//	httprouter.Param{Key: "gcmid", Value: "gcmId123"},
	//	httprouter.Param{Key: "topic", Value: "/notifications"},
	//}

	// when: I POST a message
	gcm.ServeHTTP(w, req)

	// the the result
	a.Equal("registered: /notifications\n", string(w.Body.Bytes()))
}

func TestSaveAndLoadSubscriptions(t *testing.T) {
	defer initCtrl(t)()
	defer enableDebugForMethod()()
	a := assert.New(t)

	// given: some test routes
	testRoutes := map[string]bool{
		"marvin:/foo:1234": true,
		"zappod:/bar:1212": true,
		"athur:/erde:42":   true,
	}

	routerMock := NewMockRouter(ctrl)

	kvStore := store.NewMemoryKVStore()
	routerMock.EXPECT().KVStore().Return(kvStore, nil)

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(route *server.Route) {
		// delte the route from the map, if we got it in the test
		delete(testRoutes, fmt.Sprintf("%v:%v:%v", route.UserID, route.Path, route.ApplicationID))
	}).AnyTimes()

	gcm, err := NewGCMConnector(routerMock, "/gcm/", "testApi")
	a.Nil(err)

	// when: we save the routes
	for k, _ := range testRoutes {
		splitedKey := strings.SplitN(k, ":", 3)
		userid := splitedKey[0]
		topic := splitedKey[1]
		gcmid := splitedKey[2]
		gcm.saveSubscription(userid, topic, gcmid)
	}

	// and reload the routes
	gcm.loadSubscriptions()

	time.Sleep(time.Millisecond * 100)

	// than: all expected subscriptions were called
	a.Equal(0, len(testRoutes))
}

func TestRemoveTailingSlash(t *testing.T) {
	assert.Equal(t, "/foo", removeTrailingSlash("/foo/"))
	assert.Equal(t, "/foo", removeTrailingSlash("/foo"))
}

func initCtrl(t *testing.T) func() {
	ctrl = gomock.NewController(t)
	return func() { ctrl.Finish() }
}

func enableDebugForMethod() func() {
	reset := guble.LogLevel
	guble.LogLevel = guble.LEVEL_DEBUG
	return func() { guble.LogLevel = reset }
}
