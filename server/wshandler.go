package server

import (
	"fmt"
	"log"
	"strings"

	guble "github.com/smancke/guble/guble"
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
	// Path := Ws.LocationString()
	// log.Printf("subscribe on %s", path)
	// route := NewRoute(path, 1)
	// srv.messageSouce.Subscribe(route)

	messageSouceC := make(chan []byte, 100)

	go sendLoop(srv, ws, messageSouceC)
	srv.receiveLoop(ws, messageSouceC)
}

func sendLoop(srv *WSHandler, ws WSConn, messageSouceC chan []byte) {
	for {
		msg, ok := <-messageSouceC
		if !ok {
			log.Printf("INFO: messageSouce closed the connection")
			ws.Close()
			return
		}
		if err := ws.Send(msg); err != nil {
			log.Printf("INFO: client closed the connection")
			// TODO: Handle closing the channel
			break
		}
	}
}

func (srv *WSHandler) receiveLoop(ws WSConn, messageSouceC chan []byte) {
	for {
		var message []byte

		err := ws.Receive(&message)
		if err != nil {
			log.Printf("client closed the connection")
			// TODO: how to cleanly unsubscrive from all routes
			break
		}
		if len(message) > 0 {
			msgSplit := strings.SplitN(string(message), " ", 2)
			if len(msgSplit) == 2 {
				cmd, content := msgSplit[0], msgSplit[1]

				switch cmd {
				case "subscribe":
					srv.subscribe(messageSouceC, strings.TrimSpace(content))
				case "send":
					srv.send(messageSouceC, content)
				default:
					srv.returnError(messageSouceC, "unknown command %v in %v", cmd, string(message))
				}
			} else {
				srv.returnError(messageSouceC, "no command in %v", string(message))
			}
		}
	}
}

func (srv *WSHandler) send(messageSouceC chan []byte, content string) {
	log.Printf("sending %q\n", content)
	contentSplit := strings.SplitN(string(content), " ", 2)
	if len(contentSplit) == 2 {
		srv.messageSink.HandleMessage(guble.Message{
			Path: guble.Path(contentSplit[0]),
			Body: []byte(contentSplit[1]),
		})
	} else {
		srv.returnError(messageSouceC, "no path in content %v", string(content))
	}

	srv.returnOK(messageSouceC, "sent message.")
}

func (srv *WSHandler) subscribe(messageSouceC chan []byte, path string) {
	log.Printf("subscribe: %q\n", path)
	route := NewRoute(path, messageSouceC)
	srv.messageSouce.Subscribe(route)
	srv.returnOK(messageSouceC, "subscribed-to %v", path)
}

func (srv *WSHandler) returnError(messageSouceC chan []byte, messagePattern string, params ...interface{}) {
	messageSouceC <- []byte(fmt.Sprintf("!"+messagePattern, params...))
}

func (srv *WSHandler) returnOK(messageSouceC chan []byte, messagePattern string, params ...interface{}) {
	messageSouceC <- []byte(fmt.Sprintf(">"+messagePattern, params...))
}
