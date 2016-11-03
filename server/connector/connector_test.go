package connector

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/router"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
)

type connectorMocks struct {
	router  *MockRouter
	sender  *MockSender
	queue   *MockQueue
	handler *MockResponseHandler
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
	mocks.manager.EXPECT().Create(gomock.Eq(protocol.Path("topic1")), gomock.Eq(router.RouteParams{
		"device_token": "device1",
		"user_id":      "user1",
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
	a.Equal(`{"subscribed":"topic1"}`, recorder.Body.String())
	time.Sleep(70 * time.Millisecond)
}

func TestConnector_PostSubscriptionNoMocks(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	recorder := httptest.NewRecorder()
	conn, mocks := getTestConnector(t, Config{
		Name:       "test",
		Schema:     "test",
		Prefix:     "/connector/",
		URLPattern: "/{device_token}/{user_id}/{topic:.*}",
	}, false, false)

	entriesC := make(chan [2]string)
	mocks.kvstore.EXPECT().Iterate(gomock.Eq("test"), gomock.Eq("")).Return(entriesC)
	close(entriesC)

	mocks.kvstore.EXPECT().Put(gomock.Eq("test"), gomock.Eq(GenerateKey("topic1", map[string]string{
		"device_token": "device1",
		"user_id":      "user1",
	})), gomock.Any())

	mocks.router.EXPECT().Subscribe(gomock.Any())

	err := conn.Start()
	a.NoError(err)
	defer conn.Stop()

	req, err := http.NewRequest(http.MethodPost, "/connector/device1/user1/topic1", strings.NewReader(""))
	a.NoError(err)
	conn.ServeHTTP(recorder, req)
	a.Equal(`{"subscribed":"topic1"}`, recorder.Body.String())
	time.Sleep(70 * time.Millisecond)
}

func TestConnector_DeleteSubscription(t *testing.T) {
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

	subscriber := NewMockSubscriber(testutil.MockCtrl)
	mocks.manager.EXPECT().Find(gomock.Eq(GenerateKey("topic1", map[string]string{
		"device_token": "device1",
		"user_id":      "user1",
	}))).Return(subscriber)
	mocks.manager.EXPECT().Remove(subscriber).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/connector/device1/user1/topic1", strings.NewReader(""))
	a.NoError(err)
	conn.ServeHTTP(recorder, req)
	a.Equal(`{"unsubscribed":"topic1"}`, recorder.Body.String())
	time.Sleep(70 * time.Millisecond)
}

func getTestConnector(t *testing.T, config Config, mockManager bool, mockQueue bool) (Connector, *connectorMocks) {
	a := assert.New(t)

	var (
		mManager *MockManager
		mQueue   *MockQueue
		mHandler *MockResponseHandler
	)

	mKVS := NewMockKVStore(testutil.MockCtrl)
	mRouter := NewMockRouter(testutil.MockCtrl)
	mRouter.EXPECT().KVStore().Return(mKVS, nil).AnyTimes()
	mSender := NewMockSender(testutil.MockCtrl)

	connector, err := NewConnector(mRouter, mSender, config)
	a.NoError(err)

	if mockManager {
		mManager = NewMockManager(testutil.MockCtrl)
		connector.Manager = mManager
	}
	if mockQueue {
		mHandler = NewMockResponseHandler(testutil.MockCtrl)
		mQueue = NewMockQueue(testutil.MockCtrl)
		connector.Queue = mQueue
	}

	return connector, &connectorMocks{
		mRouter,
		mSender,
		mQueue,
		mHandler,
		mManager,
		mKVS,
	}
}
