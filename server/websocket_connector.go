package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"

	"github.com/gorilla/websocket"
	"github.com/rs/xid"

	"fmt"
	"net/http"
	"strings"
	"time"
)

var webSocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSHandlerFactory struct {
	Router       PubSubSource
	MessageSink  MessageSink
	prefix       string
	messageStore store.MessageStore
}

func NewWSHandlerFactory(prefix string) *WSHandlerFactory {
	return &WSHandlerFactory{prefix: prefix}
}

func (factory *WSHandlerFactory) GetPrefix() string {
	return factory.prefix
}

func (factory *WSHandlerFactory) SetMessageEntry(messageSink MessageSink) {
	factory.MessageSink = messageSink
}

func (factory *WSHandlerFactory) SetRouter(router PubSubSource) {
	factory.Router = router
}

func (entry *WSHandlerFactory) SetMessageStore(messageStore store.MessageStore) {
	entry.messageStore = messageStore
}

func (factory *WSHandlerFactory) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := webSocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		guble.Warn("error on upgrading %v", err.Error())
		return
	}
	defer c.Close()

	_ = NewWSHandler(factory.Router, factory.MessageSink, factory.messageStore, &wsconn{c}, extractUserId(r.RequestURI)).
		Start()
}

type WSHandler struct {
	messageSouce  PubSubSource
	messageSink   MessageSink
	messageStore  store.MessageStore
	clientConn    WSConn
	applicationId string
	userId        string
	sendChannel   chan []byte
	receiver      map[guble.Path]*Receiver
}

func NewWSHandler(messageSouce PubSubSource, messageSink MessageSink, messageStore store.MessageStore, wsConn WSConn, userId string) *WSHandler {
	server := &WSHandler{
		messageSouce:  messageSouce,
		messageSink:   messageSink,
		messageStore:  messageStore,
		clientConn:    wsConn,
		applicationId: xid.New().String(),
		userId:        userId,
		sendChannel:   make(chan []byte, 10),
		receiver:      make(map[guble.Path]*Receiver),
	}
	return server
}

func (srv *WSHandler) Start() error {
	srv.sendConnectionMessage()
	go srv.sendLoop()
	srv.receiveLoop()
	return nil
}

func (srv *WSHandler) sendLoop() {
	for {
		select {
		case raw := <-srv.sendChannel:
			if guble.DebugEnabled() {
				if len(raw) < 80 {
					guble.Debug("send to client (userId=%v, applicationId=%v, totalSize=%v): %v", srv.userId, srv.applicationId, len(raw), string(raw))
				} else {
					guble.Debug("send to client (userId=%v, applicationId=%v, totalSize=%v): %v...", srv.userId, srv.applicationId, len(raw), string(raw[0:79]))
				}
			}
			if err := srv.clientConn.Send(raw); err != nil {
				guble.Info("applicationId=%v closed the connection", srv.applicationId)
				srv.cleanAndClose()
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
			guble.Info("applicationId=%v closed the connection", srv.applicationId)
			srv.cleanAndClose()
			break
		}

		//guble.Debug("websocket_connector, raw message received: %v", string(message))
		cmd, err := guble.ParseCmd(message)
		if err != nil {
			srv.sendError(guble.ERROR_BAD_REQUEST, "error parsing command. %v", err.Error())
			continue
		}
		switch cmd.Name {
		case guble.CMD_SEND:
			srv.handleSendCmd(cmd)
		case guble.CMD_RECEIVE:
			srv.handleReceiveCmd(cmd)
		case guble.CMD_CANCEL:
			srv.handleCancelCmd(cmd)
		default:
			srv.sendError(guble.ERROR_BAD_REQUEST, "unknown command %v", cmd.Name)
		}
	}
}

func (srv *WSHandler) sendConnectionMessage() {
	n := &guble.NotificationMessage{
		Name: guble.SUCCESS_CONNECTED,
		Arg:  "You are connected to the server.",
		Json: fmt.Sprintf(`{"ApplicationId": "%s", "UserId": "%s", "Time": "%s"}`, srv.applicationId, srv.userId, time.Now().Format(time.RFC3339)),
	}
	srv.sendChannel <- n.Bytes()
}

func (srv *WSHandler) handleReceiveCmd(cmd *guble.Cmd) {
	rec, err := NewReceiverFromCmd(srv.applicationId, cmd, srv.sendChannel, srv.messageSouce, srv.messageStore)

	if err != nil {
		guble.Info("client error in handleReceiveCmd: %v", err.Error())
		srv.sendError(guble.ERROR_BAD_REQUEST, err.Error())
		return
	}
	srv.receiver[rec.path] = rec
	rec.Start()
}

func (srv *WSHandler) handleCancelCmd(cmd *guble.Cmd) {
	if len(cmd.Arg) == 0 {
		srv.sendError(guble.ERROR_BAD_REQUEST, "- command requires a path argument, but non given")
		return
	}
	path := guble.Path(cmd.Arg)
	rec, exist := srv.receiver[path]
	if exist {
		rec.Stop()
		delete(srv.receiver, path)
	}
}

func (srv *WSHandler) handleSendCmd(cmd *guble.Cmd) {
	guble.Debug("sending %v", string(cmd.Bytes()))
	if len(cmd.Arg) == 0 {
		srv.sendError(guble.ERROR_BAD_REQUEST, "send command requires a path argument, but non given")
		return
	}

	args := strings.SplitN(cmd.Arg, " ", 2)
	msg := &guble.Message{
		Path: guble.Path(args[0]),
		PublisherApplicationId: srv.applicationId,
		PublisherUserId:        srv.userId,
		HeaderJson:             cmd.HeaderJson,
		Body:                   cmd.Body,
	}
	if len(args) == 2 {
		msg.PublisherMessageId = args[1]
	}

	srv.messageSink.HandleMessage(msg)

	srv.sendOK(guble.SUCCESS_SEND, msg.PublisherMessageId)
}

func (srv *WSHandler) cleanAndClose() {
	guble.Info("closing applicationId=%v", srv.applicationId)

	for path, rec := range srv.receiver {
		rec.Stop()
		delete(srv.receiver, path)
	}

	srv.clientConn.Close()
}

func (srv *WSHandler) sendError(name string, argPattern string, params ...interface{}) {
	n := &guble.NotificationMessage{
		Name:    name,
		Arg:     fmt.Sprintf(argPattern, params...),
		IsError: true,
	}
	srv.sendChannel <- n.Bytes()
}

func (srv *WSHandler) sendOK(name string, argPattern string, params ...interface{}) {
	n := &guble.NotificationMessage{
		Name:    name,
		Arg:     fmt.Sprintf(argPattern, params...),
		IsError: false,
	}
	srv.sendChannel <- n.Bytes()
}

// wsconnImpl is a Wrapper of the websocket.Conn
// implementing the interface WSConn for better testability
type wsconn struct {
	*websocket.Conn
}

func (conn *wsconn) Close() {
	conn.Conn.Close()
}

func (conn *wsconn) Send(bytes []byte) (err error) {
	return conn.Conn.WriteMessage(websocket.BinaryMessage, bytes)
}

func (conn *wsconn) Receive(bytes *[]byte) (err error) {
	_, *bytes, err = conn.Conn.ReadMessage()
	return err
}
