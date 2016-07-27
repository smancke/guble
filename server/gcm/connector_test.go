package gcm

import (
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/kvstore"
	"github.com/smancke/guble/server/router"
	"github.com/smancke/guble/testutil"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestServeHTTPSuccess(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	gcm, routerMock, _ := testGCMResponse(t, testutil.SuccessGCMResponse)

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(route *router.Route) {
		a.Equal("/notifications", string(route.Path))
		a.Equal("marvin", route.Get(userIDKey))
		a.Equal("gcmId123", route.Get(applicationIDKey))
	})

	url, _ := url.Parse("http://localhost/gcm/marvin/gcmId123/subscribe/notifications")
	// and a http context
	req := &http.Request{URL: url, Method: "POST"}
	w := httptest.NewRecorder()

	// when: I POST a message
	gcm.ServeHTTP(w, req)

	// then
	a.Equal("registered: /notifications\n", string(w.Body.Bytes()))
}

func TestGCMConnector_Check(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	gcm, _, _ := testGCMResponse(t, testutil.SuccessGCMResponse)

	done := make(chan bool)
	mockSender := testutil.CreateGcmSender(testutil.CreateRoundTripperWithJsonResponse(http.StatusOK, testutil.SuccessGCMResponse, done))
	gcm.Sender = mockSender
	err := gcm.Check()
	a.NoError(err)

	done2 := make(chan bool)
	mockSender2 := testutil.CreateGcmSender(testutil.CreateRoundTripperWithJsonResponse(http.StatusUnauthorized, "", done2))
	gcm.Sender = mockSender2
	err = gcm.Check()
	a.NotNil(err)
}

func TestServeHTTPWithErrorCases(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	gcm, _, _ := testGCMResponse(t, testutil.SuccessGCMResponse)

	url, _ := url.Parse("http://localhost/gcm/marvin/gcmId123/subscribe/notifications")
	// and a http context
	req := &http.Request{URL: url, Method: "GET"}
	w := httptest.NewRecorder()

	// do a GET instead of POST
	gcm.ServeHTTP(w, req)

	// check the result
	a.Equal("Method not allowed\n", string(w.Body.Bytes()))
	a.Equal(w.Code, http.StatusMethodNotAllowed)

	// send a new request with wrong parameters encoding
	req.Method = "POST"
	req.URL, _ = url.Parse("http://localhost/gcm/marvin/gcmId123/subscribe3/notifications")

	w2 := httptest.NewRecorder()
	gcm.ServeHTTP(w2, req)

	a.Equal("Invalid Parameters in request\n", string(w2.Body.Bytes()))
	a.Equal(w2.Code, http.StatusBadRequest)
}

func TestGCM_SaveAndLoadSubs(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	gcm, routerMock, _ := testGCMResponse(t, testutil.SuccessGCMResponse)

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
		gcmID := splitKey[2]
		initSubscription(gcm, topic, userID, gcmID, 0)
	}

	// and reload the routes
	gcm.loadSubscriptions()

	time.Sleep(50 * time.Millisecond)

	// then: all expected subscriptions were called
	a.Equal(0, len(testRoutes))
}

func TestRemoveTrailingSlash(t *testing.T) {
	assert.Equal(t, "/foo", removeTrailingSlash("/foo/"))
	assert.Equal(t, "/foo", removeTrailingSlash("/foo"))
}

func TestGCMConnector_parseParams(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	gcm, _, _ := testGCMResponse(t, testutil.SuccessGCMResponse)

	testCases := []struct {
		urlPath, userID, gcmID, topic, err string
	}{
		{"/gcm/marvin/gcmId123/subscribe/notifications", "marvin", "gcmId123", "/notifications", ""},
		{"/gcm2/marvin/gcmId123/subscribe/notifications", "", "", "", "gcm: GCM request is not starting with gcm prefix"},
		{"/gcm/marvin/gcmId123/subscrib2e/notifications", "", "", "", "gcm: GCM request third param is not subscribe"},
		{"/gcm/marvin/gcmId123subscribenotifications", "", "", "", "gcm: GCM request has wrong number of params"},
		{"/gcm/marvin/gcmId123/subscribe/notifications/alert/", "marvin", "gcmId123", "/notifications/alert", ""},
	}

	for i, c := range testCases {
		userID, gcmID, topic, err := gcm.parseParams(c.urlPath)

		//if error message is present check only the error
		if c.err != "" {
			a.NotNil(err)
			a.EqualError(err, c.err, fmt.Sprintf("Failed on testcase no=%d", i))
		} else {
			a.Equal(userID, c.userID, fmt.Sprintf("Failed on testcase no=%d", i))
			a.Equal(gcmID, c.gcmID, fmt.Sprintf("Failed on testcase no=%d", i))
			a.Equal(topic, c.topic, fmt.Sprintf("Failed on testcase no=%d", i))
			a.Nil(err, fmt.Sprintf("Failed on testcase no=%d", i))
		}
	}
	err := gcm.Stop()
	a.Nil(err)
}

func TestGCMConnector_GetPrefix(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	gcm, _, _ := testGCMResponse(t, testutil.SuccessGCMResponse)

	a.Equal(gcm.GetPrefix(), "/gcm/")
}

func TestGCMConnector_Stop(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	gcm, _, _ := testGCMResponse(t, testutil.SuccessGCMResponse)

	err := gcm.Stop()
	a.Nil(err)
	a.Equal(len(gcm.stopC), 0, "The Stop Channel should be empty")
	select {
	case _, opened := <-gcm.stopC:
		a.False(opened, "The Stop Channel should be closed")
	default:
		a.Fail("Reading from the Stop Channel should not block")
	}
}

func TestGcmConnector_StartWithMessageSending(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	gcm, _, done := testGCMResponse(t, testutil.SuccessGCMResponse)

	// put a dummy gcm message with minimum information
	route := &router.Route{
		RouteConfig: router.RouteConfig{
			RouteParams: router.RouteParams{applicationIDKey: "id"},
		},
	}
	s := newSubscription(gcm, route, 0)

	msgWithNoRecipients := newPipeMessage(s, &protocol.Message{
		ID:   uint64(4),
		Body: []byte("{id:id}"),
		Time: 1405544146,
		Path: "/gcm/marvin/gcm124/subscribe/stuff"})

	gcm.pipelineC <- msgWithNoRecipients
	// expect that the Http Server to give us a malformed message
	<-done

	//wait a little to Stop the GcmConnector
	time.Sleep(50 * time.Millisecond)
	err := gcm.Stop()
	a.NoError(err)
}

// func TestGCMConnector_BroadcastMessage(t *testing.T) {
// 	ctrl, finish := testutil.NewMockCtrl(t)
// 	defer finish()

// 	a := assert.New(t)

// 	routerMock := NewMockRouter(ctrl)

// 	kvStore := store.NewMemoryKVStore()
// 	routerMock.EXPECT().KVStore().Return(kvStore, nil)

// 	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(route *server.Route) {
// 		a.Equal("/notifications", string(route.Path))
// 		a.Equal("marvin", route.Get(userIDKey))
// 		a.Equal("gcmId123", route.Get(applicationIDKey))
// 	})

// 	gcm, err := NewGCMConnector(routerMock, "/gcm/", "testApi", 1)
// 	a.Nil(err)

// 	url, _ := url.Parse("http://localhost/gcm/marvin/gcmId123/subscribe/notifications")
// 	// and a http context
// 	req := &http.Request{URL: url, Method: "POST"}
// 	w := httptest.NewRecorder()

// 	// when: I POST a message
// 	gcm.ServeHTTP(w, req)

// 	// then
// 	a.Equal("registered: /notifications\n", string(w.Body.Bytes()))

// 	done := make(chan bool)
// 	mockSender := testutil.CreateGcmSender(
// 		testutil.CreateRoundTripperWithJsonResponse(http.StatusOK, testutil.CorrectGcmResponseMessageJSON, done))
// 	gcm.Sender = mockSender

// 	// put a broadcast message with no recipients and expect to be dropped by
// 	broadcastMessage := &protocol.Message{
// 		Message: &protocol.Message{
// 			ID:   uint64(4),
// 			Body: []byte("{id:id}"),
// 			Time: 1405544146,
// 			Path: "/gcm/broadcast"}}
// 	gcm.broadcastMessage(broadcastMessage)
// 	// wait for the message to be processed by http server
// 	<-done
// 	//wait before closing the gcm connector
// 	time.Sleep(50 * time.Millisecond)
// 	err = gcm.Stop()
// 	a.Nil(err)
// }

func TestGCMConnector_GetErrorMessageFromGcm(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)
	gcm, routerMock, done := testGCMResponse(t, testutil.ErrorGCMResponse)

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
		a.Equal("gcmCanonicalID", route.Get(applicationIDKey))
	})

	// put a dummy gcm message with minimum information
	s, err := initSubscription(gcm, "/path", "marvin", "id", 0)
	a.NoError(err)
	message := &protocol.Message{
		ID:   uint64(4),
		Body: []byte("{id:id}"),
		Time: 1405544146,
		Path: "/gcm/marvin/gcm124/subscribe/stuff",
	}

	// send the message into the subscription route channel
	s.route.Deliver(message)
	// expect that the Http Server gives us a malformed message
	<-done

	//wait before closing the gcm connector
	time.Sleep(50 * time.Millisecond)

	// stop the channel of the subscription
	s.route.Close()

	err = gcm.Stop()
	a.NoError(err)
}

func testGCMResponse(t *testing.T, jsonResponse string) (*Connector, *MockRouter, chan bool) {
	gcm, routerMock, _ := testSimpleGCM(t, false)

	err := gcm.Start()
	assert.NoError(t, err)

	done := make(chan bool)
	// return err
	mockSender := testutil.CreateGcmSender(
		testutil.CreateRoundTripperWithJsonResponse(http.StatusOK, jsonResponse, done))
	gcm.Sender = mockSender
	return gcm, routerMock, done
}

func testSimpleGCM(t *testing.T, mockStore bool) (*Connector, *MockRouter, *MockMessageStore) {
	routerMock := NewMockRouter(testutil.MockCtrl)
	kvStore := kvstore.NewMemoryKVStore()
	routerMock.EXPECT().KVStore().Return(kvStore, nil)

	gcm, err := New(routerMock, "/gcm/", "testApi", 1)
	assert.NoError(t, err)

	var storeMock *MockMessageStore
	if mockStore {
		storeMock = NewMockMessageStore(testutil.MockCtrl)
		routerMock.EXPECT().MessageStore().Return(storeMock, nil).AnyTimes()
	}

	return gcm, routerMock, storeMock
}
