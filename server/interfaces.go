package server

import (
	"github.com/rs/xid"

	guble "github.com/smancke/guble/guble"
)

type Route struct {
	Id   string
	Path guble.Path
	C    chan []byte
}

func NewRoute(path string, channel chan []byte) *Route {
	return &Route{
		Id:   xid.New().String(),
		Path: guble.Path(path),
		C:    channel,
	}
}

type PubSubSource interface {
	Subscribe(r *Route) *Route
	Unsubscribe(r *Route)
}

type MessageSink interface {
	HandleMessage(message guble.Message)
}

// WSConn is a wrapper interface for the needed functions of the websocket.Conn
// It is introduced for testability of the WSHandler
type WSConn interface {
	Close()
	Send(bytes []byte) (err error)
	Receive(bytes *[]byte) (err error)
}
