package server

import (
	"github.com/smancke/guble/protocol"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/xid"

	"bytes"
	"io/ioutil"
	"net/http"
	"strings"
)

const X_HEADER_PREFIX = "x-guble-"

type RestMessageApi struct {
	router Router
	mux    http.Handler
	prefix string
}

func NewRestMessageApi(router Router, prefix string) *RestMessageApi {
	mux := httprouter.New()
	api := &RestMessageApi{router, mux, prefix}

	p := removeTrailingSlash(prefix)
	mux.POST(p+"/message/*topic", api.PostMessage)

	return api
}

func (api *RestMessageApi) GetPrefix() string {
	return api.prefix
}

func (api *RestMessageApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.mux.ServeHTTP(w, r)
}

func (api *RestMessageApi) PostMessage(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `Can not read body`, http.StatusBadRequest)
		return
	}

	msg := &protocol.Message{
		Path:          protocol.Path(params.ByName(`topic`)),
		Body:          body,
		UserID:        q(r, `userId`),
		ApplicationID: xid.New().String(),
		MessageID:     q(r, `messageId`),
		HeaderJSON:    headersToJson(r.Header),
	}

	api.router.HandleMessage(msg)
}

// returns a query parameter
func q(r *http.Request, name string) string {
	params := r.URL.Query()[name]
	if len(params) > 0 {
		return params[0]
	}
	return ""
}

func headersToJson(header http.Header) string {
	buff := &bytes.Buffer{}
	buff.WriteString("{")

	count := 0
	for key, valueList := range header {
		if strings.HasPrefix(strings.ToLower(key), X_HEADER_PREFIX) && len(valueList) > 0 {
			if count > 0 {
				buff.WriteString(",")
			}
			buff.WriteString(`"`)
			buff.WriteString(key[len(X_HEADER_PREFIX):])
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
	if len(path) > 0 && path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}
