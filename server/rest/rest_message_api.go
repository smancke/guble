package rest

import (
	"errors"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/router"

	"github.com/rs/xid"

	"bytes"
	"io/ioutil"
	"net/http"
	"strings"
)

const xHeaderPrefix = "x-guble-"

var errNotFound = errors.New("Not Found.")

// RestMessageAPI is a struct representing a router's connector for a REST API.
type RestMessageAPI struct {
	router router.Router
	prefix string
}

// NewRestMessageAPI returns a new RestMessageAPI.
func NewRestMessageAPI(router router.Router, prefix string) *RestMessageAPI {
	return &RestMessageAPI{router, prefix}
}

// GetPrefix returns the prefix.
// It is a part of the service.endpoint implementation.
func (api *RestMessageAPI) GetPrefix() string {
	return api.prefix
}

// ServeHTTP is an http.Handler.
// It is a part of the service.endpoint implementation.
func (api *RestMessageAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `Can not read body`, http.StatusBadRequest)
		return
	}
	topic, err := api.extractTopic(r.URL.Path)
	if err != nil {
		if err == errNotFound {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Server error.", http.StatusInternalServerError)
		return
	}
	msg := &protocol.Message{
		Path:          protocol.Path(topic),
		Body:          body,
		UserID:        q(r, `userId`),
		ApplicationID: xid.New().String(),
		OptionalID:    q(r, `messageId`),
		HeaderJSON:    headersToJSON(r.Header),
	}
	api.router.HandleMessage(msg)
}

func (api *RestMessageAPI) extractTopic(path string) (string, error) {
	p := removeTrailingSlash(api.prefix) + "/message"
	if !strings.HasPrefix(path, p) {
		return "", errNotFound
	}
	// Remove "`api.prefix` + /message" and we remain with the topic
	topic := strings.TrimPrefix(path, p)
	if topic == "/" || topic == "" {
		return "", errNotFound
	}
	return topic, nil
}

// returns a query parameter
func q(r *http.Request, name string) string {
	params := r.URL.Query()[name]
	if len(params) > 0 {
		return params[0]
	}
	return ""
}

func headersToJSON(header http.Header) string {
	buff := &bytes.Buffer{}
	buff.WriteString("{")
	count := 0
	for key, valueList := range header {
		if strings.HasPrefix(strings.ToLower(key), xHeaderPrefix) && len(valueList) > 0 {
			if count > 0 {
				buff.WriteString(",")
			}
			buff.WriteString(`"`)
			buff.WriteString(key[len(xHeaderPrefix):])
			buff.WriteString(`":`)
			buff.WriteString(`"`)
			buff.WriteString(valueList[0])
			buff.WriteString(`"`)
			count++
		}
	}
	buff.WriteString("}")
	return string(buff.Bytes())
}

func removeTrailingSlash(path string) string {
	if len(path) > 1 && path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}
