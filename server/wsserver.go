package server

import (
	"github.com/smancke/guble/guble"
	"golang.org/x/net/websocket"
	"log"
	"net"
	"net/http"
	"time"
)

type WSServer struct {
	server *http.Server
	ln     net.Listener
}

func StartWSServer(addr string, newWsHandler func(wsConn WSConn) Startable) *WSServer {
	ws := &WSServer{}
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/", websocket.Handler(func(ws *websocket.Conn) {
			newWsHandler(&wsconn{ws}).Start()
		}))

		guble.Info("starting up at %v", addr)
		ws.server = &http.Server{Addr: addr, Handler: mux}
		var err error
		ws.ln, err = net.Listen("tcp", addr)
		if err != nil {
			log.Panicf("Listen: " + err.Error())
		}
		ws.server.Serve(tcpKeepAliveListener{ws.ln.(*net.TCPListener)})
		err = ws.server.ListenAndServe()
		if err != nil {
			log.Panicf("ListenAndServe: " + err.Error())
		}
	}()
	return ws
}

func (ws *WSServer) Stop() {
	guble.Info("stopping http server")
	ws.ln.Close()
}

func (ws *WSServer) GetAddr() string {
	return ws.ln.Addr().String()
}

// wsconnImpl is a Wrapper of the websocket.Conn
// implementing the interface WSConn for better testability
type wsconn struct {
	*websocket.Conn
}

func (conn *wsconn) Close() {
	conn.Conn.Close()
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

// copied from golang: net/http/server.go
// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}
