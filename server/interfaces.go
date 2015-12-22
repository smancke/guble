package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"

	"github.com/rs/xid"

	"net/http"
)

type MsgAndRoute struct {
	Message *guble.Message
	Route   *Route
}

type Route struct {
	Id                 string
	Path               guble.Path
	C                  chan MsgAndRoute
	CloseRouteByRouter chan string
	UserId             string
	ApplicationId      string
}

func NewRoute(path string, channel chan MsgAndRoute, closeRouteByRouter chan string, applicationId string, userId string) *Route {
	return &Route{
		Id:                 xid.New().String(),
		Path:               guble.Path(path),
		C:                  channel,
		CloseRouteByRouter: closeRouteByRouter,
		UserId:             userId,
		ApplicationId:      applicationId,
	}
}

type PubSubSource interface {
	Subscribe(r *Route) *Route
	Unsubscribe(r *Route)
}

type MessageSink interface {
	HandleMessage(message *guble.Message)
}

// WSConn is a wrapper interface for the needed functions of the websocket.Conn
// It is introduced for testability of the WSHandler
type WSConn interface {
	Close()
	Send(bytes []byte) (err error)
	Receive(bytes *[]byte) (err error)
}

type Startable interface {
	Start()
}

type Stopable interface {
	Stop() error
}

type SetRouter interface {
	SetRouter(router PubSubSource)
}

// interface for modules, which need a MessageEntry set
type SetMessageEntry interface {
	SetMessageEntry(messageSink MessageSink)
}

type Endpoint interface {
	http.Handler
	GetPrefix() string
}

// Interface for modules, which need a Key Value store set,
// for storing their data
type SetKVStore interface {
	SetKVStore(kvStore store.KVStore)
}
