package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"

	"net/http"
)

type MsgAndRoute struct {
	Message *guble.Message
	Route   *Route
}

type Route struct {
	Path          guble.Path
	C             chan MsgAndRoute
	UserId        string
	ApplicationId string
}

func (r Route) equals(other Route) bool {
	return r.Path == other.Path &&
		r.UserId == other.UserId &&
		r.ApplicationId == other.ApplicationId
}

func NewRoute(path string, channel chan MsgAndRoute, applicationId string, userId string) *Route {
	return &Route{
		Path:          guble.Path(path),
		C:             channel,
		UserId:        userId,
		ApplicationId: applicationId,
	}
}

type PubSubSource interface {
	Subscribe(r *Route) (*Route, error)
	Unsubscribe(r *Route)
}

type MessageSink interface {
	HandleMessage(message *guble.Message) error
}

// WSConnection is a wrapper interface for the needed functions of the websocket.Conn
// It is introduced for testability of the WSHandler
type WSConnection interface {
	Close()
	Send(bytes []byte) (err error)
	Receive(bytes *[]byte) (err error)
}

type Startable interface {
	Start() error
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

// Interface for modules, which need access to the message store
type SetMessageStore interface {
	SetMessageStore(messageStore store.MessageStore)
}

type SetAccessManager interface {
	SetAccessManager(accessManager AccessManager)
}

type AccessType int

const (
	READ AccessType = iota
	WRITE
)

type AccessManager interface {
	AccessAllowed(accessType AccessType, userId string, path guble.Path) bool
}
