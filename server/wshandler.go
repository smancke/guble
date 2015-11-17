package server

import (
	"golang.org/x/net/websocket"
	"log"
	"net/http"
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

func (srv *WSHandler) Start(listen string) {

	go func() {
		log.Printf("starting up at %v", listen)
		http.Handle("/", websocket.Handler(func(ws *websocket.Conn) {
			srv.handleConnection(ws)
		}))

		err := http.ListenAndServe(listen, nil)
		if err != nil {
			log.Panicf("ListenAndServe: " + err.Error())
		}
	}()
}

func (srv *WSHandler) Stop() {
}

func (srv *WSHandler) handleConnection(ws *websocket.Conn) {
	path := ws.Config().Location.String()
	log.Printf("subscribe on %s", path)
	route := NewRoute(path, srv.routeChannelSize)
	srv.messageSouce.Subscribe(route)

	go sendLoop(srv, ws, route)
	srv.receiveLoop(ws, route)
}

func sendLoop(srv *WSHandler, ws *websocket.Conn, route *Route) {
	for {
		msg, ok := <-route.C
		if !ok {
			log.Printf("INFO: messageSouce closed the connection %q", route.Path)
			ws.Close()
			return
		}
		if err := websocket.Message.Send(ws, msg); err != nil {
			log.Printf("INFO: client closed the connection for path %q", route.Path)
			srv.messageSouce.Unsubscribe(route)
			break
		}
	}
}

func (srv *WSHandler) receiveLoop(ws *websocket.Conn, route *Route) {
	for {
		var message []byte
		//log.Printf("DEBUG: receiveLoop -> waiting for messages")

		err := websocket.Message.Receive(ws, &message)
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
