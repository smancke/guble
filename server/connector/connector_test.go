package connector

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/router"
	"github.com/cosminrentea/gobbler/testutil"
	"github.com/stretchr/testify/assert"
)

type connectorMocks struct {
	router  *MockRouter
	sender  *MockSender
	queue   *MockQueue
	manager *MockManager
	kvstore *MockKVStore
}

// Ensure the subscription is started when posting
func TestConnector_PostSubscription(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	recorder := httptest.NewRecorder()
	conn, mocks := getTestConnector(t, Config{
		Name:       "test",
		Schema:     "test",
		Prefix:     "/connector/",
		URLPattern: "/{device_token}/{user_id}/{topic:.*}",
	}, true, false)

	mocks.manager.EXPECT().Load().Return(nil)
	mocks.manager.EXPECT().List().Return(make([]Subscriber, 0))
	err := conn.Start()
	a.NoError(err)
	defer conn.Stop()

	subscriber := NewMockSubscriber(testutil.MockCtrl)
	mocks.manager.EXPECT().Create(gomock.Eq(protocol.Path("/topic1")), gomock.Eq(router.RouteParams{
		"device_token": "device1",
		"user_id":      "user1",
		"connector":    "test",
	})).Return(subscriber, nil)

	subscriber.EXPECT().Loop(gomock.Any(), gomock.Any())
	r := router.NewRoute(router.RouteConfig{
		Path: protocol.Path("topic1"),
		RouteParams: router.RouteParams{
			"device_token": "device1",
			"user_id":      "user1",
		},
	})
	subscriber.EXPECT().Route().Return(r)
	mocks.router.EXPECT().Subscribe(gomock.Eq(r)).Return(r, nil)

	req, err := http.NewRequest(http.MethodPost, "/connector/device1/user1/topic1", strings.NewReader(""))
	a.NoError(err)
	conn.ServeHTTP(recorder, req)
	a.Equal(`{"subscribed":"/topic1"}`, recorder.Body.String())
	time.Sleep(100 * time.Millisecond)
}

func TestConnector_PostSubscriptionNoMocks(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	recorder := httptest.NewRecorder()
	conn, mocks := getTestConnector(t, Config{
		Name:       "name",
		Schema:     "schema",
		Prefix:     "/connector/",
		URLPattern: "/{device_token}/{user_id}/{topic:.*}",
	}, false, false)

	entriesC := make(chan [2]string)
	mocks.kvstore.EXPECT().Iterate(gomock.Eq("schema"), gomock.Eq("")).Return(entriesC)
	close(entriesC)

	mocks.kvstore.EXPECT().Put(gomock.Eq("schema"), gomock.Eq(GenerateKey("/topic1", map[string]string{
		"device_token": "device1",
		"user_id":      "user1",
		"connector":    "name",
	})), gomock.Any())

	mocks.router.EXPECT().Subscribe(gomock.Any())

	err := conn.Start()
	a.NoError(err)
	defer conn.Stop()

	req, err := http.NewRequest(http.MethodPost, "/connector/device1/user1/topic1", strings.NewReader(""))
	a.NoError(err)
	conn.ServeHTTP(recorder, req)
	a.Equal(`{"subscribed":"/topic1"}`, recorder.Body.String())
	time.Sleep(100 * time.Millisecond)
}

func TestConnector_DeleteSubscription(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	recorder := httptest.NewRecorder()
	conn, mocks := getTestConnector(t, Config{
		Name:       "name",
		Schema:     "schema",
		Prefix:     "/connector/",
		URLPattern: "/{device_token}/{user_id}/{topic:.*}",
	}, true, false)

	subscriber := NewMockSubscriber(testutil.MockCtrl)
	mocks.manager.EXPECT().Find(gomock.Eq(GenerateKey("/topic1", map[string]string{
		"device_token": "device1",
		"user_id":      "user1",
		"connector":    "name",
	}))).Return(subscriber)
	mocks.manager.EXPECT().Remove(subscriber).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/connector/device1/user1/topic1", strings.NewReader(""))
	a.NoError(err)
	conn.ServeHTTP(recorder, req)
	a.Equal(`{"unsubscribed":"/topic1"}`, recorder.Body.String())
	time.Sleep(200 * time.Millisecond)
}

func TestConnector_GetList_And_Getters(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	recorder := httptest.NewRecorder()
	conn, mocks := getTestConnector(t, Config{
		Name:       "test",
		Schema:     "test",
		Prefix:     "/connector/",
		URLPattern: "/{device_token}/{user_id}/{topic:.*}",
	}, true, false)

	req, err := http.NewRequest(http.MethodGet, "/connector/", strings.NewReader(""))
	a.NoError(err)

	conn.ServeHTTP(recorder, req)
	expectedJSON := `{"error":"Missing filters"}`
	a.JSONEq(expectedJSON, recorder.Body.String())
	a.Equal(http.StatusBadRequest, recorder.Code)

	a.Equal("/connector/", conn.GetPrefix())
	a.Equal(mocks.manager, conn.Manager())
	a.Equal(nil, conn.ResponseHandler())
}

func TestConnector_GetListWithFilters(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	recorder := httptest.NewRecorder()
	conn, mocks := getTestConnector(t, Config{
		Name:       "test",
		Schema:     "test",
		Prefix:     "/connector/",
		URLPattern: "/{device_token}/{user_id}/{topic:.*}",
	}, true, false)

	mocks.manager.EXPECT().Filter(gomock.Eq(map[string]string{
		"filter1": "value1",
		"filter2": "value2",
	})).Return([]Subscriber{})

	req, err := http.NewRequest(
		http.MethodGet,
		"/connector/?filter1=value1&filter2=value2",
		strings.NewReader(""))
	a.NoError(err)

	conn.ServeHTTP(recorder, req)
}

func TestConnector_StartWithSubscriptions(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	conn, mocks := getTestConnector(t, Config{
		Name:       "test",
		Schema:     "test",
		Prefix:     "/connector/",
		URLPattern: "/{device_token}/{user_id}/{topic:.*}",
	}, false, false)

	entriesC := make(chan [2]string)
	mocks.kvstore.EXPECT().Iterate(gomock.Eq("test"), gomock.Eq("")).Return(entriesC)
	close(entriesC)
	mocks.kvstore.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Times(4)

	err := conn.Start()
	a.NoError(err)

	routes := make([]*router.Route, 0, 4)
	mocks.router.EXPECT().Subscribe(gomock.Any()).Do(func(r *router.Route) (*router.Route, error) {
		routes = append(routes, r)
		return r, nil
	}).Times(4)

	// create subscriptions
	createSubscriptions(t, conn, 4)
	time.Sleep(100 * time.Millisecond)

	mocks.sender.EXPECT().Send(gomock.Any()).Return(nil, nil).Times(4)

	// send message in route channel
	for i, r := range routes {
		r.Deliver(&protocol.Message{
			ID:   uint64(i),
			Path: protocol.Path("/topic"),
			Body: []byte("test body"),
		}, true)
	}

	time.Sleep(100 * time.Millisecond)

	err = conn.Stop()
	a.NoError(err)
}

func TestConnector_Substitute(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	conn, mocks := getTestConnector(t, Config{
		Name:       "test",
		Schema:     "test",
		Prefix:     "/connector/",
		URLPattern: "/{device_token}/{user_id}/{topic:.*}",
	}, false, false)

	entriesC := make(chan [2]string)
	mocks.kvstore.EXPECT().Iterate(gomock.Eq("test"), gomock.Eq("")).Return(entriesC)
	close(entriesC)
	mocks.kvstore.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Times(4)

	err := conn.Start()
	a.NoError(err)

	routes := make([]*router.Route, 0, 4)
	mocks.router.EXPECT().Subscribe(gomock.Any()).Do(func(r *router.Route) (*router.Route, error) {
		routes = append(routes, r)
		return r, nil
	}).Times(4)

	// create subscriptions
	createSubscriptions(t, conn, 4)
	time.Sleep(100 * time.Millisecond)

	postBody := `{
			"field":"device_token",
			"old_value":"device1",
			"new_value":"asgasgasgagasgaasg2"
			}
	`
	mocks.kvstore.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/connector"+SubstitutePath, strings.NewReader(postBody))
	conn.ServeHTTP(recorder, req)

	a.Equal(http.StatusOK, recorder.Code)
	a.Equal(`{"modified":"1"}`, recorder.Body.String())
}

func TestConnector_SubstituteWrongPostBody(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	conn, mocks := getTestConnector(t, Config{
		Name:       "test",
		Schema:     "test",
		Prefix:     "/connector/",
		URLPattern: "/{device_token}/{user_id}/{topic:.*}",
	}, false, false)

	entriesC := make(chan [2]string)
	mocks.kvstore.EXPECT().Iterate(gomock.Eq("test"), gomock.Eq("")).Return(entriesC)
	close(entriesC)
	mocks.kvstore.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).Times(4)

	err := conn.Start()
	a.NoError(err)

	routes := make([]*router.Route, 0, 4)
	mocks.router.EXPECT().Subscribe(gomock.Any()).Do(func(r *router.Route) (*router.Route, error) {
		routes = append(routes, r)
		return r, nil
	}).Times(4)

	// create subscriptions
	createSubscriptions(t, conn, 4)
	time.Sleep(100 * time.Millisecond)

	postBody := `{
			"field_invalid":"device_token",
			"old_value":"device1",
			"new_value":"asgasgasgagasgaasg2"
			}
	`
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/connector"+SubstitutePath, strings.NewReader(postBody))
	conn.ServeHTTP(recorder, req)

	a.Equal(http.StatusBadRequest, recorder.Code)
}

func createSubscriptions(t *testing.T, conn Connector, count int) {
	a := assert.New(t)
	for i := 1; i <= count; i++ {
		recorder := httptest.NewRecorder()
		r, err := http.NewRequest(
			http.MethodPost,
			fmt.Sprintf("/connector/device%d/user%d/topic", i, i),
			strings.NewReader(""))
		a.NoError(err)
		conn.ServeHTTP(recorder, r)
		a.Equal(200, recorder.Code)
		a.Equal(`{"subscribed":"/topic"}`, recorder.Body.String())
	}
}

func TestConnector_StartAndStopWithoutSubscribers(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	conn, mocks := getTestConnector(t, Config{
		Name:       "test",
		Schema:     "test",
		Prefix:     "/connector/",
		URLPattern: "/{device_token}/{user_id}/{topic:.*}",
	}, true, true)
	mocks.manager.EXPECT().Load().Return(nil)
	mocks.manager.EXPECT().List().Return(nil)
	mocks.queue.EXPECT().Start().Return(nil)
	mocks.queue.EXPECT().Stop().Return(nil)

	err := conn.Start()
	a.NoError(err)

	err = conn.Stop()
	a.NoError(err)
}

func getTestConnector(t *testing.T, config Config, mockManager bool, mockQueue bool) (Connector, *connectorMocks) {
	a := assert.New(t)

	var (
		mManager *MockManager
		mQueue   *MockQueue
	)

	mKVS := NewMockKVStore(testutil.MockCtrl)
	mRouter := NewMockRouter(testutil.MockCtrl)
	mRouter.EXPECT().KVStore().Return(mKVS, nil).AnyTimes()
	mSender := NewMockSender(testutil.MockCtrl)

	conn, err := NewConnector(mRouter, mSender, config)
	a.NoError(err)

	if mockManager {
		mManager = NewMockManager(testutil.MockCtrl)
		conn.(*connector).manager = mManager
	}
	if mockQueue {
		mQueue = NewMockQueue(testutil.MockCtrl)
		conn.(*connector).queue = mQueue
	}

	return conn, &connectorMocks{
		mRouter,
		mSender,
		mQueue,
		mManager,
		mKVS,
	}
}
