package rest

import (
	"errors"
	"fmt"

	"github.com/azer/snakecase"

	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/router"

	"github.com/rs/xid"

	"bytes"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
)

const (
	XHeaderPrefix     = "x-guble-"
	filterPrefix      = "filter"
	subscribersPrefix = "/subscribers"
)

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
	if r.Method == http.MethodHead {
		return
	}

	if r.Method == http.MethodGet {
		log.WithField("url", r.URL.Path).Debug("GET")

		topic, err := api.extractTopic(r.URL.Path, subscribersPrefix)
		if err != nil {
			log.WithError(err).Error("Extracting topic failed")
			if err == errNotFound {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "Server error.", http.StatusInternalServerError)
			return
		}

		resp, err := api.router.GetSubscribers(topic)
		w.Header().Set("Content-Type", "application/json")

		_, err = w.Write(resp)
		if err != nil {
			log.WithField("error", err.Error()).Error("Writing to byte stream failed")
			http.Error(w, "Server error.", http.StatusInternalServerError)
			return
		}
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Can not read body", http.StatusBadRequest)
		return
	}

	topic, err := api.extractTopic(r.URL.Path, "/message")
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
		UserID:        q(r, "userId"),
		ApplicationID: xid.New().String(),
		HeaderJSON:    headersToJSON(r.Header),
	}

	// add filters
	api.setFilters(r, msg)

	api.router.HandleMessage(msg)
	fmt.Fprintf(w, "OK")
}

func (api *RestMessageAPI) extractTopic(path string, requestTypeTopicPrefix string) (string, error) {
	p := removeTrailingSlash(api.prefix) + requestTypeTopicPrefix
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

// setFilters sets a field found in the format `filterCamelCaseField` in the
// query of the request to underscore format on the message filters
func (api *RestMessageAPI) setFilters(r *http.Request, msg *protocol.Message) {
	for name, values := range r.URL.Query() {
		if strings.HasPrefix(name, filterPrefix) && len(values) > 0 {
			msg.SetFilter(filterName(name), values[0])
		}
	}
}

// returns a query parameter
func q(r *http.Request, name string) string {
	params := r.URL.Query()[name]
	if len(params) > 0 {
		return params[0]
	}
	return ""
}

// transform from filterCamelCase to camel_case
func filterName(name string) string {
	return snakecase.SnakeCase(strings.TrimPrefix(name, filterPrefix))
}

func headersToJSON(header http.Header) string {
	buff := &bytes.Buffer{}
	buff.WriteString("{")
	count := 0
	for key, valueList := range header {
		if strings.HasPrefix(strings.ToLower(key), XHeaderPrefix) && len(valueList) > 0 {
			if count > 0 {
				buff.WriteString(",")
			}
			buff.WriteString(`"`)
			buff.WriteString(key[len(XHeaderPrefix):])
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
