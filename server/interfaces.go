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
