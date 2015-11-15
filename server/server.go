package server

import (
	"golang.org/x/net/websocket"
	"log"
	"net/http"
)

type Server struct {
	mux *MsgMultiplexer
}

func NewServer(mux *MsgMultiplexer) *Server {
	server := &Server{
		mux: mux,
	}
	return server
}

func (srv *Server) handleConnection(ws *websocket.Conn) {
	path := ws.Config().Location.String()
	log.Printf("subscribe on %s", path)
	route := srv.mux.AddNewRoute(path)

	go sendLoop(srv, ws, route)
	srv.receiveLoop(ws, route)
}

func sendLoop(srv *Server, ws *websocket.Conn, route Route) {
	for {
		msg, ok := <-route.C
		if !ok {
			log.Printf("INFO: mux closed the connection %q", route.Path)
			ws.Close()
			return
		}
		if err := websocket.Message.Send(ws, msg); err != nil {
			log.Printf("INFO: client closed the connection for path %q", route.Path)
			srv.mux.RemoveRoute(route)
			break
		}
	}
}

func (srv *Server) receiveLoop(ws *websocket.Conn, route Route) {
	for {
		var message []byte
		//log.Printf("DEBUG: receiveLoop -> waiting for messages")

		err := websocket.Message.Receive(ws, &message)
		if err != nil {
			log.Printf("client closed the connection for path %q", route.Path)
			srv.mux.RemoveRoute(route)
			break
		}
		if len(message) > 0 {
			messageCopy := make([]byte, len(message))
			copy(messageCopy, message)

			srv.mux.HandleMessage(Message{
				path: route.Path,
				body: messageCopy,
			})
		}
	}
}

func (srv *Server) Start(listen string) {

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

func (srv *Server) Stop() {
}
