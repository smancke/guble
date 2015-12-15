package server

import (
	"fmt"
	"github.com/rs/xid"

	guble "github.com/smancke/guble/guble"
	"strings"
)

type WSHandler struct {
	messageSouce        PubSubSource
	messageSink         MessageSink
	clientConn          WSConn
	applicationId       string
	userId              string
	messagesToSend      chan *guble.Message
	routeClosed         chan string
	notificationsToSend chan *guble.NotificationMessage
	subscriptions       map[guble.Path]*Route
}

func NewWSHandler(messageSouce PubSubSource, messageSink MessageSink, wsConn WSConn, userId string) *WSHandler {
	server := &WSHandler{
		messageSink:         messageSink,
		messageSouce:        messageSouce,
		clientConn:          wsConn,
		applicationId:       xid.New().String(),
		userId:              userId,
		messagesToSend:      make(chan *guble.Message, 100),
		notificationsToSend: make(chan *guble.NotificationMessage, 100),
		routeClosed:         make(chan string, 100),
		subscriptions:       make(map[guble.Path]*Route),
	}
	return server
}

func (srv *WSHandler) Start() {
	go srv.sendLoop()
	srv.receiveLoop()
}

func (srv *WSHandler) sendLoop() {
	for {
		select {
		case msg, ok := <-srv.messagesToSend:
			if !ok {
				guble.Info("messageSouce closed the connection -> closing the websocket connection to applicationId=%v", srv.applicationId)
				srv.clientConn.Close()
				return
			}
			if guble.DebugEnabled() {
				guble.Debug("deliver message to applicationId=%v: %v", srv.applicationId, msg.MetadataLine())
			}
			if guble.InfoEnabled() {
				guble.Info("sending message: %v", msg.MetadataLine())
			}
			if err := srv.clientConn.Send(msg.Bytes()); err != nil {
				guble.Info("applicationId=%v closed the connection", srv.applicationId)
				srv.cleanAndClose()
				break
			}
		case msg, ok := <-srv.notificationsToSend:
			if !ok {
				guble.Info("messageSouce closed the connection -> closing the websocket connection to applicationId=%v", srv.applicationId)
				srv.cleanAndClose()
				return
			}
			if err := srv.clientConn.Send(msg.Bytes()); err != nil {
				guble.Info("applicationId=%v closed the connection", srv.applicationId)
				srv.cleanAndClose()
				break
			}
		case closedRouteId := <-srv.routeClosed:
			guble.Info("INFO: router closed route %v -> closing the websocket connection to applicationId=%v", closedRouteId, srv.applicationId)
			srv.cleanAndClose()
			return
		}
	}
}

func (srv *WSHandler) receiveLoop() {
	var message []byte
	for {
		err := srv.clientConn.Receive(&message)
		if err != nil {
			guble.Info("applicationId=%v closed the connection", srv.applicationId)
			srv.cleanAndClose()
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
	guble.Info("sending %q\n", string(cmd.Body))
	if len(cmd.Arg) == 0 {
		srv.returnError(guble.ERROR_BAD_REQUEST, "send command requires a path argument, but non given", cmd.Name)
		return
	}
	args := strings.SplitN(cmd.Arg, " ", 2)
	msg := &guble.Message{
		Path: guble.Path(args[0]),
		PublisherApplicationId: srv.applicationId,
		PublisherUserId:        srv.userId,
		Body:                   cmd.Body,
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
	route := NewRoute(cmd.Arg, srv.messagesToSend, srv.routeClosed, srv.applicationId)
	srv.messageSouce.Subscribe(route)
	srv.subscriptions[route.Path] = route
	srv.returnOK("subscribed-to", cmd.Arg)
}

func (srv *WSHandler) cleanAndClose() {
	guble.Info("closing applicationId=%v", srv.applicationId)

	for _, route := range srv.subscriptions {
		srv.messageSouce.Unsubscribe(route)
		delete(srv.subscriptions, route.Path)
	}

	srv.clientConn.Close()
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
