package server

import (
	"github.com/gorilla/websocket"
	"github.com/smancke/guble/guble"
	"log"
	"net"
	"net/http"
	"time"
)

type WSServer struct {
	server *http.Server
	ln     net.Listener
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func StartWSServer(addr string, newWsHandler func(wsConn WSConn) Startable) *WSServer {
	ws := &WSServer{}
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			c, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				guble.Warn("error on upgrading %v", err.Error())
				return
			}
			defer c.Close()
			newWsHandler(&wsconn{c}).Start()
		})

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
	return conn.LocationString()
}

func (conn *wsconn) Send(bytes []byte) (err error) {
	return conn.Conn.WriteMessage(websocket.BinaryMessage, bytes)
}

func (conn *wsconn) Receive(bytes *[]byte) (err error) {
	_, *bytes, err = conn.Conn.ReadMessage()
	return err
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
