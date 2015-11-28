package server

import (
	"github.com/smancke/guble/guble"
	"golang.org/x/net/websocket"
	"log"
	"net/http"
)

type WSServer struct {
	server *http.Server
}

func StartWSServer(listen string, srv *WSHandler) *WSServer {
	ws := &WSServer{}
	go func() {
		guble.Info("starting up at %v", listen)
		mux := http.NewServeMux()
		mux.Handle("/", websocket.Handler(func(ws *websocket.Conn) {
			handleConnection(ws, srv)
		}))

		ws.server = &http.Server{Addr: listen, Handler: mux}
		err := ws.server.ListenAndServe()

		if err != nil {
			log.Panicf("ListenAndServe: " + err.Error())
		}
	}()
	return ws
}

func handleConnection(ws *websocket.Conn, srv *WSHandler) {
	srv.HandleNewConnection(&wsconn{ws})
}

func (ws *WSServer) Stop() {
	guble.Info("stopping http server !!!!TODO .. implement")
	// TODO .. implement
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
