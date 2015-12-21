package gcm

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"

	"github.com/golang/mock/gomock"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"

	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

var ctrl *gomock.Controller

func TestPostMessage(t *testing.T) {
	defer initCtrl(t)()

	a := assert.New(t)

	// given:  a rest api with a message sink
	routerMock := NewMockPubSubSource(ctrl)
	kvStore := store.NewMemoryKVStore()
	gcm := NewGCMConnector("/gcm/")
	gcm.SetRouter(routerMock)
	gcm.SetKVStore(kvStore)

	url, _ := url.Parse("http://localhost/gcm/marvin/xyz1234/notifications")
	// and a http context
	req := &http.Request{URL: url}
	w := httptest.NewRecorder()

	params := httprouter.Params{
		httprouter.Param{Key: "userid", Value: "marvin"},
		httprouter.Param{Key: "gcmid", Value: "xyz123"},
		httprouter.Param{Key: "topic", Value: "notifications"},
	}

	// when: I POST a message
	gcm.Register(w, req, params)

	// the the result
	a.Equal("registered: /notifications\n", string(w.Body.Bytes()))
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

// Mock of PubSubSource interface
type MockPubSubSource struct {
	ctrl     *gomock.Controller
	recorder *_MockPubSubSourceRecorder
}

// Recorder for MockPubSubSource (not exported)
type _MockPubSubSourceRecorder struct {
	mock *MockPubSubSource
}

func NewMockPubSubSource(ctrl *gomock.Controller) *MockPubSubSource {
	mock := &MockPubSubSource{ctrl: ctrl}
	mock.recorder = &_MockPubSubSourceRecorder{mock}
	return mock
}

func (_m *MockPubSubSource) EXPECT() *_MockPubSubSourceRecorder {
	return _m.recorder
}

func (_m *MockPubSubSource) Subscribe(_param0 *server.Route) *server.Route {
	ret := _m.ctrl.Call(_m, "Subscribe", _param0)
	ret0, _ := ret[0].(*server.Route)
	return ret0
}

func (_mr *_MockPubSubSourceRecorder) Subscribe(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Subscribe", arg0)
}

func (_m *MockPubSubSource) Unsubscribe(_param0 *server.Route) {
	_m.ctrl.Call(_m, "Unsubscribe", _param0)
}

func (_mr *_MockPubSubSourceRecorder) Unsubscribe(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Unsubscribe", arg0)
}
