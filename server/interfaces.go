package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"

	"github.com/smancke/guble/server/auth"
	"net/http"
)

// PubSubSource interface provides mechanism for PubSub messaging
type PubSubSource interface {
	KVStore() (store.KVStore, error)
	AccessManager() (auth.AccessManager, error)
	MessageStore() (store.MessageStore, error)

	Subscribe(r *Route) (*Route, error)
	Unsubscribe(r *Route)
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

// Endpoint adds a HTTP handler for the `GetPrefix()` to the webserver
type Endpoint interface {
	http.Handler
	GetPrefix() string
}
