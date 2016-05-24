package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"

	"github.com/smancke/guble/server/auth"
	"net/http"
)

// MsgAndRoute is a wrapper that provides the message and the route together.
// It is useful for sending both over a channel.
type MsgAndRoute struct {
	Message *guble.Message
	Route   *Route
}

type Route struct {
	Path          guble.Path
	C             chan MsgAndRoute
	UserID        string // UserId that subscribed or pushes messages to the router
	ApplicationID string
}

func (r Route) equals(other Route) bool {
	return r.Path == other.Path &&
		r.UserID == other.UserID &&
		r.ApplicationID == other.ApplicationID
}

// NewRoute creates a new route pointer
func NewRoute(
	path string,
	channel chan MsgAndRoute,
	applicationID string,
	userID string) *Route {
	return &Route{
		Path:          guble.Path(path),
		C:             channel,
		UserID:        userID,
		ApplicationID: applicationID,
	}
}

// PubSubSource interface provides mechanism for PubSub messaging
type PubSubSource interface {
	KVStore() (store.KVStore, error)

	Subscribe(r *Route) (*Route, error)
	Unsubscribe(r *Route)
}

// MessageSink interface allows for sending/pushing messages
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

// Startable interface for modules which provide a start mechanism
type Startable interface {
	Start() error
}

// Stopable interface for modules which provide a stop mechanism
type Stopable interface {
	Stop() error
}

// SetRouter interface for modules which require a `Router`
type SetRouter interface {
	SetRouter(router PubSubSource)
}

// SetMessageEntry interface for modules which need a MessageEntry set
type SetMessageEntry interface {
	SetMessageEntry(messageSink MessageSink)
}

// Endpoint adds a HTTP handler for the `GetPrefix()` to the webserver
type Endpoint interface {
	http.Handler
	GetPrefix() string
}

// SetMessageStore for modules which need access to the message store
type SetMessageStore interface {
	SetMessageStore(messageStore store.MessageStore)
}

// SetAccessManager for modules which need access to the access manager
type SetAccessManager interface {
	SetAccessManager(accessManager auth.AccessManager)
}
