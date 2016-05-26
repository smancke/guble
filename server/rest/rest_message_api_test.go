package rest

import (
	"github.com/smancke/guble/protocol"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestServerHTTP(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	// given:  a rest api with a message sink
	routerMock := NewMockRouter(ctrl)
	api := NewRestMessageAPI(routerMock, "/api")

	url, _ := url.Parse("http://localhost/api/message/my/topic?userId=marvin&messageId=42")

	// and a http context
	req := &http.Request{
		Method: http.MethodPost,
		URL:    url,
		Body:   ioutil.NopCloser(bytes.NewReader(testBytes)),
		Header: http.Header{},
	}
	w := &httptest.ResponseRecorder{}

	// then i expect
	routerMock.EXPECT().HandleMessage(gomock.Any()).Do(func(msg *protocol.Message) {
		a.Equal(testBytes, msg.Body)
		a.Equal("{}", msg.HeaderJSON)
		a.Equal("/my/topic", string(msg.Path))
		a.True(len(msg.ApplicationID) > 0)
		a.Equal("42", msg.MessageID)
		a.Equal("marvin", msg.UserID)
	})

	// when: I POST a message
	api.ServeHTTP(w, req)

}

// Server should return an 405 Method Not Allowed in case method request is not POST
func TestServeHTTP_GetError(t *testing.T) {
	a := assert.New(t)
	api := NewRestMessageAPI(nil, "/api")

	url, _ := url.Parse("http://localhost/api/message/my/topic?userId=marvin&messageId=42")
	// and a http context
	req := &http.Request{
		Method: http.MethodGet,
		URL:    url,
		Body:   ioutil.NopCloser(bytes.NewReader(testBytes)),
		Header: http.Header{},
	}
	w := &httptest.ResponseRecorder{}

	// when: I POST a message
	api.ServeHTTP(w, req)

	a.Equal(http.StatusMethodNotAllowed, w.Code)
}

func TestHeadersToJSON(t *testing.T) {
	a := assert.New(t)

	// empty header
	a.Equal(`{}`, headersToJSON(http.Header{}))

	// simple head
	jsonString := headersToJSON(http.Header{
		X_HEADER_PREFIX + "a": []string{"b"},
		"foo": []string{"b"},
		X_HEADER_PREFIX + "x": []string{"y"},
		"bar": []string{"b"},
	})

	header := make(map[string]string)
	err := json.Unmarshal([]byte(jsonString), &header)
	a.NoError(err)

	a.Equal(2, len(header))
	a.Equal("b", header["a"])
	a.Equal("y", header["x"])
}

func TestRemoveTailingSlash(t *testing.T) {
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
		topic, err := api.extractTopic(c.path)
		m := "Assertion failed for path: " + c.path

		if c.err == nil {
			a.Equal(c.topic, topic, m)
		} else {
			a.NotNil(err, m)
			a.Equal(c.err, err, m)
		}
	}
}
