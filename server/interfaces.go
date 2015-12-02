package server

import (
	"github.com/rs/xid"

	guble "github.com/smancke/guble/guble"
)

type Route struct {
	Id                 string
	Path               guble.Path
	C                  chan *guble.Message
	CloseRouteByRouter chan string
	ApplicationId      string
}

func NewRoute(path string, channel chan *guble.Message, closeRouteByRouter chan string, applicationId string) *Route {
	return &Route{
		Id:                 xid.New().String(),
		Path:               guble.Path(path),
		C:                  channel,
		CloseRouteByRouter: closeRouteByRouter,
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
