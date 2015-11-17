package server

import (
	"golang.org/x/net/websocket"
	"log"
	"net/http"
)

func StartWSServer(listen string, srv *WSHandler) {
	go func() {
		log.Printf("starting up at %v", listen)
		http.Handle("/", websocket.Handler(func(ws *websocket.Conn) {
			handleConnection(ws, srv)
		}))

		err := http.ListenAndServe(listen, nil)
		if err != nil {
			log.Panicf("ListenAndServe: " + err.Error())
		}
	}()
}

func handleConnection(ws *websocket.Conn, srv *WSHandler) {
	srv.HandleNewConnection(&wsconn{ws})
}

// wsconnImpl is a Wrapper of the websocket.Conn
// implementing the interface WSConn for better testability
type wsconn struct {
	*websocket.Conn
}

func (conn *wsconn) Close() {
	conn.Close()
}

func (conn *wsconn) LocationString() string {
	return conn.Config().Location.String()
}

func (conn *wsconn) Send(bytes []byte) (err error) {
	return websocket.Message.Send(conn.Conn, bytes)
}

func (conn *wsconn) Receive(bytes *[]byte) (err error) {
	return websocket.Message.Receive(conn.Conn, bytes)
}
