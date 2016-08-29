package webserver

import (
	"net"
	"net/http"
	"strings"
	"time"
)

// WebServer is a struct representing a HTTP Server (using a net.Listener and a ServeMux multiplexer).
type WebServer struct {
	server *http.Server
	ln     net.Listener
	mux    *http.ServeMux
	addr   string
}

// New returns a new WebServer.
func New(addr string) *WebServer {
	return &WebServer{
		mux:  http.NewServeMux(),
		addr: addr,
	}
}

// Start the WebServer (implementing service.startable interface).
func (ws *WebServer) Start() (err error) {
	logger.WithField("address", ws.addr).Info("Http server is starting up on address")

	ws.server = &http.Server{Addr: ws.addr, Handler: ws.mux}
	ws.ln, err = net.Listen("tcp", ws.addr)
	if err != nil {
		return
	}

	go func() {
		err = ws.server.Serve(tcpKeepAliveListener{TCPListener: ws.ln.(*net.TCPListener)})
		if err != nil && !strings.HasSuffix(err.Error(), "use of closed network connection") {
			logger.WithError(err).Error("ListenAndServe")
		}
		logger.WithField("address", ws.addr).Info("Http server stopped")
	}()
	return
}

// Stop the WebServer (implementing service.stopable interface).
func (ws *WebServer) Stop() (err error) {
	if ws.ln != nil {
		err = ws.ln.Close()
	}

	// reset the mux
	ws.mux = http.NewServeMux()
	return
}

// Handle the given prefix using the given handler.
// It is a part of the service.endpoint interface.
func (ws *WebServer) Handle(prefix string, handler http.Handler) {
	ws.mux.Handle(prefix, handler)
}

// GetAddr returns the address on which the WebServer is listening.
// It is a part of the service.endpoint interface.
func (ws *WebServer) GetAddr() string {
	if ws.ln == nil {
		return "::unknown::"
	}
	return ws.ln.Addr().String()
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
	tc.SetKeepAlivePeriod(10 * time.Second)
	return tc, nil
}
