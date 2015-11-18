package server

import (
	"github.com/rs/xid"
)

type Message struct {
	id   int64
	path Path
	body []byte
}

type Path string

type Route struct {
	Id   string
	Path Path
	C    chan []byte
}

func NewRoute(path string, routeChannelSize int) *Route {
	return &Route{
		Id:   xid.New().String(),
		Path: Path(path),
		C:    make(chan []byte, routeChannelSize),
	}
}

type PubSubSource interface {
	Subscribe(r *Route) *Route
	Unsubscribe(r *Route)
}

type MessageSink interface {
	HandleMessage(message Message)
}

// WSConn is a wrapper interface for the needed functions of the websocket.Conn
// It is introduced for testability of the WSHandler
type WSConn interface {
	Close()
	LocationString() string
	Send(bytes []byte) (err error)
	Receive(bytes *[]byte) (err error)
}
