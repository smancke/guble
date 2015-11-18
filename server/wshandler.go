package server

import (
	"log"
)

type WSHandler struct {
	messageSouce     PubSubSource
	messageSink      MessageSink
	routeChannelSize int
}

func NewWSHandler(messageSouce PubSubSource, messageSink MessageSink) *WSHandler {
	server := &WSHandler{
		messageSink:      messageSink,
		messageSouce:     messageSouce,
		routeChannelSize: 100,
	}
	return server
}

func (srv *WSHandler) HandleNewConnection(ws WSConn) {
	path := ws.LocationString()

	log.Printf("subscribe on %s", path)
	route := NewRoute(path, 1)
	srv.messageSouce.Subscribe(route)

	go sendLoop(srv, ws, route)
	srv.receiveLoop(ws, route)
}

func sendLoop(srv *WSHandler, ws WSConn, route *Route) {
	for {
		msg, ok := <-route.C
		if !ok {
			log.Printf("INFO: messageSouce closed the connection %q", route.Path)
			ws.Close()
			return
		}
		if err := ws.Send(msg); err != nil {
			log.Printf("INFO: client closed the connection for path %q", route.Path)
			srv.messageSouce.Unsubscribe(route)
			break
		}
	}
}

func (srv *WSHandler) receiveLoop(ws WSConn, route *Route) {
	for {
		var message []byte
		//log.Printf("DEBUG: receiveLoop -> waiting for messages")

		err := ws.Receive(&message)
		if err != nil {
			log.Printf("client closed the connection for path %q", route.Path)
			srv.messageSouce.Unsubscribe(route)
			break
		}
		if len(message) > 0 {
			messageCopy := make([]byte, len(message))
			copy(messageCopy, message)

			srv.messageSink.HandleMessage(Message{
				path: route.Path,
				body: messageCopy,
			})
		}
	}
}
