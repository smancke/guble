package server

import (
	"fmt"
	"log"

	guble "github.com/smancke/guble/guble"
	"strings"
)

type WSHandler struct {
	messageSouce        PubSubSource
	messageSink         MessageSink
	clientConn          WSConn
	messagesToSend      chan *guble.Message
	notificationsToSend chan *guble.NotificationMessage
}

func NewWSHandler(messageSouce PubSubSource, messageSink MessageSink, wsConn WSConn) *WSHandler {
	server := &WSHandler{
		messageSink:         messageSink,
		messageSouce:        messageSouce,
		clientConn:          wsConn,
		messagesToSend:      make(chan *guble.Message, 100),
		notificationsToSend: make(chan *guble.NotificationMessage, 100),
	}
	return server
}

func (srv *WSHandler) Start() {
	// Path := Ws.LocationString()
	// log.Printf("subscribe on %s", path)
	// route := NewRoute(path, 1)
	// srv.messageSouce.Subscribe(route)

	go srv.sendLoop()
	srv.receiveLoop()
}

func (srv *WSHandler) sendLoop() {
	for {
		select {
		case msg, ok := <-srv.messagesToSend:
			if !ok {
				log.Printf("INFO: messageSouce closed the connection")
				srv.clientConn.Close()
				return
			}
			if err := srv.clientConn.Send(msg.Bytes()); err != nil {
				log.Printf("INFO: client closed the connection")
				// TODO: Handle closing the channel
				break
			}
		case msg, ok := <-srv.notificationsToSend:
			if !ok {
				log.Printf("INFO: messageSouce closed the connection")
				srv.clientConn.Close()
				return
			}
			if err := srv.clientConn.Send(msg.Bytes()); err != nil {
				log.Printf("INFO: client closed the connection")
				// TODO: Handle closing the channel
				break
			}
		}
	}
}

func (srv *WSHandler) receiveLoop() {
	var message []byte
	for {
		err := srv.clientConn.Receive(&message)
		if err != nil {
			log.Printf("client closed the connection")
			// TODO: how to cleanly unsubscrive from all routes
			break
		}

		cmd, err := guble.ParseCmd(message)
		if err != nil {
			srv.returnError(guble.ERROR_BAD_REQUEST, "error parsing command. %v", err.Error())
			continue
		}
		switch cmd.Name {
		case guble.CMD_SEND:
			srv.send(cmd)
		case guble.CMD_SUBSCRIBE:
			srv.subscribe(cmd)
		default:
			srv.returnError(guble.ERROR_BAD_REQUEST, "unknown command %v", cmd.Name)
		}
	}
}

func (srv *WSHandler) send(cmd *guble.Cmd) {
	log.Printf("sending %q\n", string(cmd.Body))
	if len(cmd.Arg) == 0 {
		srv.returnError(guble.ERROR_BAD_REQUEST, "send command requires a path argument, but non given", cmd.Name)
		return
	}
	args := strings.SplitN(cmd.Arg, " ", 2)
	msg := &guble.Message{
		Path: guble.Path(args[0]),
		Body: cmd.Body,
	}
	if len(args) == 2 {
		msg.PublisherMessageId = args[1]
	}
	srv.messageSink.HandleMessage(msg)

	srv.returnOK(guble.SUCCESS_SEND, msg.PublisherMessageId)
}

func (srv *WSHandler) subscribe(cmd *guble.Cmd) {
	if len(cmd.Arg) == 0 {
		srv.returnError(guble.ERROR_BAD_REQUEST, "subscribe command requires a path argument, but non given", cmd.Name)
		return
	}
	log.Printf("subscribe: %q\n", cmd.Arg)
	route := NewRoute(cmd.Arg, srv.messagesToSend)
	srv.messageSouce.Subscribe(route)
	srv.returnOK("subscribed-to", cmd.Arg)
}

func (srv *WSHandler) returnError(name string, argPattern string, params ...interface{}) {
	srv.notificationsToSend <- &guble.NotificationMessage{
		Name:    name,
		Arg:     fmt.Sprintf(argPattern, params...),
		IsError: true,
	}
}

func (srv *WSHandler) returnOK(name string, argPattern string, params ...interface{}) {
	srv.notificationsToSend <- &guble.NotificationMessage{
		Name:    name,
		Arg:     fmt.Sprintf(argPattern, params...),
		IsError: false,
	}
}
