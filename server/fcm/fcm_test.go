package fcm

import (
	"github.com/Bogh/gcm"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/kvstore"
	"github.com/smancke/guble/server/router"
	"github.com/smancke/guble/testutil"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestConnector_ServeHTTPSuccess(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	g, routerMock, _ := testFCMResponse(t, testutil.SuccessFCMResponse)

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(route *router.Route) {
		a.Equal("/notifications", string(route.Path))
		a.Equal("marvin", route.Get(userIDKey))
		a.Equal("fcmId123", route.Get(applicationIDKey))
	})

	time.Sleep(100 * time.Millisecond)

	postSubscription(t, g, "marvin", "fcmId123", "notifications")
}

func TestConnector_ServeHTTPWithErrorCases(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	g, _, _ := testFCMResponse(t, testutil.SuccessFCMResponse)

	u, _ := url.Parse("http://localhost/gcm/marvin/fcmId123/subscribe/notifications")
	// and a http context
	req := &http.Request{URL: u, Method: "HEAD"}
	w := httptest.NewRecorder()

	// do a GET instead of POST
	g.ServeHTTP(w, req)

	// check the result
	a.Equal("{\"error\":\"method not allowed\"}\n", string(w.Body.Bytes()))
	a.Equal(w.Code, http.StatusMethodNotAllowed)

	// send a new request with wrong parameters encoding
	req.Method = "POST"
	req.URL, _ = u.Parse("http://localhost/gcm/marvin/fcmId123/subscribe3/notifications")

	w2 := httptest.NewRecorder()
	g.ServeHTTP(w2, req)

	a.Equal("{\"error\":\"invalid parameters in request\"}\n", string(w2.Body.Bytes()))
	a.Equal(w2.Code, http.StatusBadRequest)
}

//TODO Cosmin Bogdan test should be re-enabled after Check() works and is a public func
//func TestConnector_Check(t *testing.T) {
//	_, finish := testutil.NewMockCtrl(t)
//	defer finish()
//
//	a := assert.New(t)
//	gcm, _, _ := testGCMResponse(t, testutil.SuccessGCMResponse)
//
//	done := make(chan bool)
//	mockSender := testutil.CreateGcmSender(
//		testutil.CreateRoundTripperWithJsonResponse(
//			http.StatusOK, testutil.SuccessGCMResponse, done))
//	gcm.Sender = mockSender
//	err := gcm.check()
//	a.NoError(err)
//
//	done2 := make(chan bool)
//	mockSender2 := testutil.CreateGcmSender(
//		testutil.CreateRoundTripperWithJsonResponse(
//			http.StatusUnauthorized, "", done2))
//	gcm.Sender = mockSender2
//	a.Error(gcm.check())
//}

func TestFCM_SaveAndLoadSubs(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	g, routerMock, _ := testFCMResponse(t, testutil.SuccessFCMResponse)

	// given: some test routes
	testRoutes := map[string]bool{
		"marvin:/foo:1234": true,
		"zappod:/bar:1212": true,
		"athur:/erde:42":   true,
	}

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(route *router.Route) {
		// delete the route from the map, if we got it in the test
		delete(testRoutes, fmt.Sprintf("%v:%v:%v", route.Get(userIDKey), route.Path, route.Get(applicationIDKey)))
	}).AnyTimes()

	// when: we save the routes
	for k := range testRoutes {
		splitKey := strings.SplitN(k, ":", 3)
		userID := splitKey[0]
		topic := splitKey[1]
		fcmID := splitKey[2]
		initSubscription(g, topic, userID, fcmID, 0, true)
	}

	// and reload the routes
	g.loadSubscriptions()

	time.Sleep(50 * time.Millisecond)

	// then: all expected subscriptions were called
	a.Equal(0, len(testRoutes))
}

func TestRemoveTrailingSlash(t *testing.T) {
	assert.Equal(t, "/foo", removeTrailingSlash("/foo/"))
	assert.Equal(t, "/foo", removeTrailingSlash("/foo"))
}

func TestConnector_parseParams(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	g, _, _ := testFCMResponse(t, testutil.SuccessFCMResponse)

	testCases := []struct {
		urlPath, userID, fcmID, topic, err string
	}{
		{"/gcm/marvin/fcmId123/subscribe/notifications", "marvin", "fcmId123", "/notifications", ""},
		{"/gcm2/marvin/fcmId123/subscribe/notifications", "", "", "", "FCM request is not starting with correct prefix"},
		{"/gcm/marvin/fcmId123/subscrib2e/notifications", "", "", "", "FCM request third param is not subscribe"},
		{"/gcm/marvin/fcmId123subscribenotifications", "", "", "", "FCM request has wrong number of params"},
		{"/gcm/marvin/fcmId123/subscribe/notifications/alert/", "marvin", "fcmId123", "/notifications/alert", ""},
	}

	for i, c := range testCases {
		userID, fcmID, unparsed, err := g.parseUserIDAndDeviceId(c.urlPath)
		var topic string
		if err == nil {
			topic, err = g.parseTopic(unparsed)
		}
		//if error message is present check only the error
		if c.err != "" {
			a.NotNil(err)
			a.EqualError(err, c.err, fmt.Sprintf("Failed on testcase no=%d", i))
		} else {
			a.Equal(userID, c.userID, fmt.Sprintf("Failed on testcase no=%d", i))
			a.Equal(fcmID, c.fcmID, fmt.Sprintf("Failed on testcase no=%d", i))
			a.Equal(topic, c.topic, fmt.Sprintf("Failed on testcase no=%d", i))
			a.Nil(err, fmt.Sprintf("Failed on testcase no=%d", i))
		}
	}
	err := g.Stop()
	a.Nil(err)
}

func TestConnector_GetPrefix(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	g, _, _ := testFCMResponse(t, testutil.SuccessFCMResponse)

	a.Equal(g.GetPrefix(), "/gcm/")
}

func TestConnector_Stop(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	g, _, _ := testFCMResponse(t, testutil.SuccessFCMResponse)

	err := g.Stop()
	a.Nil(err)
	a.Equal(len(g.stopC), 0, "The Stop Channel should be empty")
	select {
	case _, opened := <-g.stopC:
		a.False(opened, "The Stop Channel should be closed")
	default:
		a.Fail("Reading from the Stop Channel should not block")
	}
}

func TestConnector_StartWithMessageSending(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	g, _, done := testFCMResponse(t, testutil.SuccessFCMResponse)

	// put a dummy FCM message with minimum information
	route := &router.Route{
		RouteConfig: router.RouteConfig{
			RouteParams: router.RouteParams{applicationIDKey: "id"},
		},
	}
	s := newSubscription(g, route, 0)

	msgWithNoRecipients := newSubscriptionMessage(s, &protocol.Message{
		ID:   uint64(4),
		Body: []byte("{id:id}"),
		Time: 1405544146,
		Path: "/gcm/marvin/fcm124/subscribe/stuff"})

	g.pipelineC <- msgWithNoRecipients
	// expect that the Http Server to give us a malformed message
	<-done

	//wait a little to Stop the FCM Connector
	time.Sleep(50 * time.Millisecond)
	err := g.Stop()
	a.NoError(err)
}

func TestConnector_GetErrorMessageFromFCM(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	g, routerMock, done := testFCMResponse(t, testutil.ErrorFCMResponse)

	// expect the route unsubscribed from removeSubscription
	routerMock.EXPECT().Unsubscribe(gomock.Any()).Do(func(route *router.Route) {
		a.Equal("/path", string(route.Path))
		a.Equal("id", route.Get(applicationIDKey))
	})

	// expect the route subscribe with the new canonicalId from replaceSubscriptionWithCanonicalID
	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(route *router.Route) {
		a.Equal("/path", string(route.Path))
		a.Equal("marvin", route.Get(userIDKey))
		a.Equal("id", route.Get(applicationIDKey))
	})
	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(route *router.Route) {
		a.Equal("/path", string(route.Path))
		a.Equal("marvin", route.Get(userIDKey))
		appid := route.Get(applicationIDKey)
		a.Equal("fcmCanonicalID", appid)
	})
	msMock := NewMockMessageStore(testutil.MockCtrl)

	routerMock.EXPECT().MessageStore().Return(msMock, nil)
	msMock.EXPECT().MaxMessageID(gomock.Any()).Return(uint64(4), nil)

	// Wait for subscriptions to finish loading
	time.Sleep(100 * time.Millisecond)

	// put a dummy FCM message with minimum information
	s, err := initSubscription(g, "/path", "marvin", "id", 0, true)
	a.NoError(err)
	time.Sleep(100 * time.Millisecond)

	message := &protocol.Message{
		ID:   uint64(4),
		Body: []byte("{id:id}"),
		Time: 1405544146,
		Path: "/gcm/marvin/fcm124/subscribe/stuff",
	}

	// send the message into the subscription route channel
	s.route.Deliver(message)
	// expect that the Http Server gives us a malformed message
	<-done

	//wait before closing the FCM connector
	time.Sleep(500 * time.Millisecond)

	// stop the channel of the subscription
	// s.route.Close()

	err = g.Stop()
	a.NoError(err)
}

func TestConnector_Subscribe(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	g, routerMock, _ := testSimpleFCM(t, true)

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(route *router.Route) {
		a.Equal("/baskets", string(route.Path))
		a.Equal("user1", route.Get(userIDKey))
		a.Equal("fcm1", route.Get(applicationIDKey))
	})

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(route *router.Route) {
		a.Equal("/baskets", string(route.Path))
		a.Equal("user2", route.Get(userIDKey))
		a.Equal("fcm2", route.Get(applicationIDKey))
	})

	postSubscription(t, g, "user1", "fcm1", "baskets")
	a.Equal(len(g.subscriptions), 1)

	postSubscription(t, g, "user2", "fcm2", "baskets")
	a.Equal(len(g.subscriptions), 2)
}

func TestConnector_RetrieveNoSubscriptions(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	g, _, _ := testSimpleFCM(t, true)

	w := httptest.NewRecorder()
	u := fmt.Sprintf("http://localhost/gcm/%s/%s/subscribe/", "user01", "fcm01")
	req, err := http.NewRequest(http.MethodGet, u, nil)
	a.NoError(err)

	g.ServeHTTP(w, req)
	a.Equal(http.StatusOK, w.Code)
	var bytes bytes.Buffer
	err = json.NewEncoder(&bytes).Encode([]string{})
	a.NoError(err)
	a.Equal(bytes.String(), w.Body.String())
}

func TestConnector_RetrieveSubscriptions(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	g, routerMock, _ := testSimpleFCM(t, true)

	routerMock.EXPECT().Subscribe(gomock.Any())
	routerMock.EXPECT().Subscribe(gomock.Any())

	w := httptest.NewRecorder()
	u := fmt.Sprintf("http://localhost/gcm/%s/%s/subscribe/%s", "user01", "fcm01", "test")

	//subscribe first user
	req, err := http.NewRequest(http.MethodPost, u, nil)
	a.NoError(err)
	g.ServeHTTP(w, req)

	//subscribe second user
	w = httptest.NewRecorder()
	u2 := fmt.Sprintf("http://localhost/gcm/%s/%s/subscribe/%s", "user01", "fcm01", "test2")
	req, err = http.NewRequest(http.MethodPost, u2, nil)
	a.NoError(err)
	g.ServeHTTP(w, req)
	a.Equal(http.StatusOK, w.Code)

	// retrieve all subscriptions
	w = httptest.NewRecorder()
	u3 := fmt.Sprintf("http://localhost/gcm/%s/%s/subscribe/", "user01", "fcm01")
	req, err = http.NewRequest(http.MethodGet, u3, nil)
	a.NoError(err)
	g.ServeHTTP(w, req)
	a.Equal(http.StatusOK, w.Code)

	var bytes bytes.Buffer
	expTopics := []string{"test2", "test"}
	sort.Strings(expTopics)
	err = json.NewEncoder(&bytes).Encode(expTopics)
	a.NoError(err)
	a.JSONEq(bytes.String(), w.Body.String())
}

func TestConnector_Unsubscribe(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	g, routerMock, _ := testSimpleFCM(t, true)
	var deletedRoute *router.Route

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(route *router.Route) {
		deletedRoute = route
		a.Equal("/baskets", string(route.Path))
		a.Equal("user1", route.Get(userIDKey))
		a.Equal("fcm1", route.Get(applicationIDKey))

	})

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(route *router.Route) {
		a.Equal("/baskets", string(route.Path))
		a.Equal("user2", route.Get(userIDKey))
		a.Equal("fcm2", route.Get(applicationIDKey))
	})

	postSubscription(t, g, "user1", "fcm1", "baskets")
	a.Equal(len(g.subscriptions), 1)

	postSubscription(t, g, "user2", "fcm2", "baskets")
	a.Equal(len(g.subscriptions), 2)

	routerMock.EXPECT().Unsubscribe(deletedRoute)

	deleteSubscription(t, g, "user1", "fcm1", "baskets")
	a.Equal(len(g.subscriptions), 1)

	remainingKey := composeSubscriptionKey("/baskets", "user2", "fcm2")

	a.Equal("user2", g.subscriptions[remainingKey].route.Get(userIDKey))
	a.Equal("fcm2", g.subscriptions[remainingKey].route.Get(applicationIDKey))
	a.Equal("/baskets", string(g.subscriptions[remainingKey].route.Path))
}

func TestConnector_SubscriptionExists(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	g, routerMock, _ := testSimpleFCM(t, true)

	routerMock.EXPECT().Subscribe(gomock.Any())

	w := httptest.NewRecorder()
	u := fmt.Sprintf("http://localhost/gcm/%s/%s/subscribe/%s", "user01", "fcm01", "/test")

	req, err := http.NewRequest(http.MethodPost, u, nil)
	a.NoError(err)

	g.ServeHTTP(w, req)
	w = httptest.NewRecorder()
	g.ServeHTTP(w, req)

	a.Equal(http.StatusOK, w.Code)
	a.Equal("{\"error\":\"subscription already exists\"}", w.Body.String())
}

func TestFCMFormatMessage(t *testing.T) {
	mockCtrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	var subRoute *router.Route

	connector, routerMock, _ := testSimpleFCM(t, false)
	fcmSenderMock := NewMockSender(mockCtrl)
	connector.Sender = fcmSenderMock
	connector.Start()
	defer connector.Stop()
	time.Sleep(50 * time.Millisecond)

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(route *router.Route) (*router.Route, error) {
		subRoute = route
		return route, nil
	})

	postSubscription(t, connector, "user01", "device01", "topic")

	// send a fully formated GCM message
	m := &protocol.Message{
		Path: "/topic",
		ID:   1,
		Body: []byte(fullFCMMessage),
	}

	if !a.NotNil(subRoute) {
		return
	}

	doneC := make(chan bool)

	fcmSenderMock.EXPECT().Send(gomock.Any()).Do(func(m *gcm.Message) (*gcm.Response, error) {
		a.NotNil(m.Notification)
		a.Equal("TEST", m.Notification.Title)
		a.Equal("notification body", m.Notification.Body)
		a.Equal("ic_notification_test_icon", m.Notification.Icon)
		a.Equal("estimated_arrival", m.Notification.ClickAction)

		a.NotNil(m.Data)
		if a.Contains(m.Data, "field1") {
			a.Equal("value1", m.Data["field1"])
		}
		if a.Contains(m.Data, "field2") {
			a.Equal("value2", m.Data["field2"])
		}

		doneC <- true
		return nil, nil
	}).Return(&gcm.Response{}, nil)

	subRoute.Deliver(m)
	select {
	case <-doneC:
	case <-time.After(100 * time.Millisecond):
		a.Fail("Message not received by FCM")
	}

	m = &protocol.Message{
		Path: "/topic",
		ID:   1,
		Body: []byte(`plain body`),
	}

	fcmSenderMock.EXPECT().Send(gomock.Any()).Do(func(m *gcm.Message) (*gcm.Response, error) {
		a.Nil(m.Notification)

		a.NotNil(m.Data)
		a.Contains(m.Data, "message")

		doneC <- true
		return nil, nil
	}).Return(&gcm.Response{}, nil)

	subRoute.Deliver(m)
	select {
	case <-doneC:
	case <-time.After(100 * time.Millisecond):
		a.Fail("Message not received by FCM")
	}
}

func testFCMResponse(t *testing.T, jsonResponse string) (*Connector, *MockRouter, chan bool) {
	g, routerMock, _ := testSimpleFCM(t, false)

	err := g.Start()
	assert.NoError(t, err)

	done := make(chan bool)
	// return err
	mockSender := testutil.CreateGcmSender(
		testutil.CreateRoundTripperWithJsonResponse(http.StatusOK, jsonResponse, done))
	g.Sender = mockSender
	return g, routerMock, done
}

func testSimpleFCM(t *testing.T, mockStore bool) (*Connector, *MockRouter, *MockMessageStore) {
	routerMock := NewMockRouter(testutil.MockCtrl)
	routerMock.EXPECT().Cluster().Return(nil)
	kvStore := kvstore.NewMemoryKVStore()
	routerMock.EXPECT().KVStore().Return(kvStore, nil)

	key := "testApi"
	nWorkers := 1
	endpoint := ""
	g, err := New(
		routerMock,
		"/gcm/",
		Config{
			APIKey:   &key,
			Workers:  &nWorkers,
			Endpoint: &endpoint,
		})
	assert.NoError(t, err)

	var storeMock *MockMessageStore
	if mockStore {
		storeMock = NewMockMessageStore(testutil.MockCtrl)
		routerMock.EXPECT().MessageStore().Return(storeMock, nil).AnyTimes()
	}

	return g, routerMock, storeMock
}

func postSubscription(t *testing.T, fcmConn *Connector, userID, gcmID, topic string) {
	a := assert.New(t)
	u := fmt.Sprintf("http://localhost/gcm/%s/%s/subscribe/%s", userID, gcmID, topic)
	req, err := http.NewRequest(http.MethodPost, u, nil)
	a.NoError(err)
	w := httptest.NewRecorder()

	fcmConn.ServeHTTP(w, req)

	a.Equal(fmt.Sprintf(`{"subscribed":"/%s"}`, topic), string(w.Body.Bytes()))
}

func deleteSubscription(t *testing.T, fcmConn *Connector, userID, gcmID, topic string) {
	a := assert.New(t)
	u := fmt.Sprintf("http://localhost/gcm/%s/%s/subscribe/%s", userID, gcmID, topic)
	req, err := http.NewRequest(http.MethodDelete, u, nil)
	a.NoError(err)
	w := httptest.NewRecorder()

	fcmConn.ServeHTTP(w, req)

	a.Equal(fmt.Sprintf(`{"unsubscribed":"/%s"}`, topic), string(w.Body.Bytes()))
}
