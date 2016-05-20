package server

import (
	"github.com/smancke/guble/guble"

	"github.com/golang/mock/gomock"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"

	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestPostMessage(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	// given:  a rest api with a message sink
	messageSink := NewMockMessageSink(ctrl)
	api := NewRestMessageApi(messageSink, "/api")

	url, _ := url.Parse("http://localhost/api/message/my/topic?userId=marvin&messageId=42")
	// and a http context
	req := &http.Request{
		URL:    url,
		Body:   ioutil.NopCloser(bytes.NewReader(testBytes)),
		Header: http.Header{},
	}
	w := &httptest.ResponseRecorder{}

	params := httprouter.Params{
		httprouter.Param{Key: "topic", Value: "/my/topic"},
	}

	// then i expect
	messageSink.EXPECT().HandleMessage(gomock.Any()).Do(func(msg *guble.Message) {
		a.Equal(testBytes, msg.Body)
		a.Equal("{}", msg.HeaderJSON)
		a.Equal("/my/topic", string(msg.Path))
		a.True(len(msg.PublisherApplicationId) > 0)
		a.Equal("42", msg.PublisherMessageId)
		a.Equal("marvin", msg.PublisherUserId)
	})

	// when: I POST a message
	api.PostMessage(w, req, params)

}

func TestHeadersToJson(t *testing.T) {
	a := assert.New(t)

	// empty header
	a.Equal(`{}`, headersToJson(http.Header{}))

	// simple head
	jsonString := headersToJson(http.Header{
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
}
