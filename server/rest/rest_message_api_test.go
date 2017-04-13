package rest

import (
	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/testutil"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"bytes"
	"encoding/json"
	"fmt"
	"github.com/cosminrentea/go-uuid"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

var testBytes = []byte("test")

func TestServerHTTP_MethodNotAllowed(t *testing.T) {
	a := assert.New(t)
	api := NewRestMessageAPI(nil, "/api")

	u, _ := url.Parse("http://localhost/api/message/my/topic?userId=marvin&messageId=42")
	// and a http context
	req := &http.Request{
		Method: http.MethodDelete,
		URL:    u,
		Body:   ioutil.NopCloser(bytes.NewReader(testBytes)),
		Header: http.Header{},
	}
	w := &httptest.ResponseRecorder{}

	// when: I POST a message
	api.ServeHTTP(w, req)

	//then
	a.Equal(http.StatusMethodNotAllowed, w.Code)

}

func TestServerHTTP(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	// given:  a rest api with a message sink
	routerMock := NewMockRouter(ctrl)
	api := NewRestMessageAPI(routerMock, "/api")

	u, _ := url.Parse("http://localhost/api/message/my/topic?userId=marvin&messageId=42")

	header := http.Header{}
	genUUID, _ := go_uuid.New()
	header.Add(XHeaderPrefix+"correlation-id", genUUID)
	// and a http context
	req := &http.Request{
		Method: http.MethodPost,
		URL:    u,
		Body:   ioutil.NopCloser(bytes.NewReader(testBytes)),
		Header: header,
	}
	w := &httptest.ResponseRecorder{}

	// then i expect
	routerMock.EXPECT().HandleMessage(gomock.Any()).Do(func(msg *protocol.Message) {
		a.Equal(testBytes, msg.Body)
		a.JSONEq(fmt.Sprintf(`{"Correlation-Id": "%s"}`, genUUID), msg.HeaderJSON)
		a.Equal(genUUID, msg.CorrelationID())
		a.Equal("/my/topic", string(msg.Path))
		a.True(len(msg.ApplicationID) > 0)
		a.Nil(msg.Filters)
		a.Equal("marvin", msg.UserID)
	})

	// when: I POST a message
	api.ServeHTTP(w, req)
}

// Server should return an 405 Method Not Allowed in case method request is not POST
func TestServeHTTP_GetError(t *testing.T) {
	a := assert.New(t)
	api := NewRestMessageAPI(nil, "/api")

	u, _ := url.Parse("http://localhost/api/message/my/topic?userId=marvin&messageId=42")
	// and a http context
	req := &http.Request{
		Method: http.MethodGet,
		URL:    u,
		Body:   ioutil.NopCloser(bytes.NewReader(testBytes)),
		Header: http.Header{},
	}
	w := &httptest.ResponseRecorder{}

	// when: I POST a message
	api.ServeHTTP(w, req)

	//then
	a.Equal(http.StatusNotFound, w.Code)
}

// Server should return an 405 Method Not Allowed in case method request is not POST
func TestServeHTTP_GetSubscribers(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	routerMock := NewMockRouter(testutil.MockCtrl)
	api := NewRestMessageAPI(routerMock, "/api")
	routerMock.EXPECT().GetSubscribers(gomock.Any()).Return([]byte("{}"), nil)
	u, _ := url.Parse("http://localhost/api/subscribers/mytopic")
	// and a http context
	req := &http.Request{
		Method: http.MethodGet,
		URL:    u,
	}
	w := &httptest.ResponseRecorder{}

	// when: I POST a message
	api.ServeHTTP(w, req)

	//then
	a.Equal(http.StatusOK, w.Code)
}

func TestHeadersToJSON(t *testing.T) {
	a := assert.New(t)

	// empty header
	a.Equal(`{}`, headersToJSON(http.Header{}))

	// simple head
	jsonString := headersToJSON(http.Header{
		XHeaderPrefix + "a": []string{"b"},
		"foo":               []string{"b"},
		XHeaderPrefix + "x": []string{"y"},
		"bar":               []string{"b"},
	})

	header := make(map[string]string)
	err := json.Unmarshal([]byte(jsonString), &header)
	a.NoError(err)

	a.Equal(2, len(header))
	a.Equal("b", header["a"])
	a.Equal("y", header["x"])
}

func TestRemoveTrailingSlash(t *testing.T) {
	assert.Equal(t, "/foo", removeTrailingSlash("/foo/"))
	assert.Equal(t, "/foo", removeTrailingSlash("/foo"))
	assert.Equal(t, "/", removeTrailingSlash("/"))
}

func TestExtractTopic(t *testing.T) {
	a := assert.New(t)

	api := NewRestMessageAPI(nil, "/api")

	cases := []struct {
		path, topic string
		err         error
	}{
		{"/api/message/my/topic", "/my/topic", nil},
		{"/api/message/", "", errNotFound},
		{"/api/message", "", errNotFound},
		{"/api/invalid/request", "", errNotFound},
	}

	for _, c := range cases {
		topic, err := api.extractTopic(c.path, "/message")
		m := "Assertion failed for path: " + c.path

		if c.err == nil {
			a.Equal(c.topic, topic, m)
		} else {
			a.NotNil(err, m)
			a.Equal(c.err, err, m)
		}
	}
}

func TestRestMessageAPI_setFilters(t *testing.T) {
	a := assert.New(t)

	body := bytes.NewBufferString("")
	req, err := http.NewRequest(
		http.MethodPost,
		"http://localhost/api/message/topic?filterUserID=user01&filterDeviceID=ABC&filterDummyCamelCase=dummy_value",
		body)
	a.NoError(err)

	api := &RestMessageAPI{}
	msg := &protocol.Message{}

	api.setFilters(req, msg)

	a.NotNil(msg.Filters)
	if a.Contains(msg.Filters, "user_id") {
		a.Equal("user01", msg.Filters["user_id"])
	}
	if a.Contains(msg.Filters, "device_id") {
		a.Equal("ABC", msg.Filters["device_id"])
	}
	if a.Contains(msg.Filters, "dummy_camel_case") {
		a.Equal("dummy_value", msg.Filters["dummy_camel_case"])
	}
}

func TestRestMessageAPI_SetFiltersWhenServing(t *testing.T) {
	testutil.SkipIfDisabled(t)
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	body := bytes.NewBufferString("")
	req, err := http.NewRequest(
		http.MethodPost,
		"http://localhost/test/message/topic?filterUserID=user01&filterDeviceID=ABC&filterDummyCamelCase=dummy_value",
		body)
	a.NoError(err)

	routerMock := NewMockRouter(testutil.MockCtrl)
	api := NewRestMessageAPI(routerMock, "/test/")
	recorder := httptest.NewRecorder()

	routerMock.EXPECT().HandleMessage(gomock.Any()).Do(func(msg *protocol.Message) error {
		a.NotNil(msg.Filters)
		if a.Contains(msg.Filters, "user_id") {
			a.Equal("user01", msg.Filters["user_id"])
		}
		if a.Contains(msg.Filters, "device_id") {
			a.Equal("ABC", msg.Filters["device_id"])
		}
		if a.Contains(msg.Filters, "dummy_camel_case") {
			a.Equal("dummy_value", msg.Filters["dummy_camel_case"])
		}

		return nil
	})

	api.ServeHTTP(recorder, req)

	time.Sleep(10 * time.Millisecond)
}
