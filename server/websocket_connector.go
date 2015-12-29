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

	NewWSHandler(factory.Router, factory.MessageSink, factory.messageStore, &wsconn{c}, extractUserId(r.RequestURI)).
		Start()
}

type WSHandler struct {
	messageSouce           PubSubSource
	messageSink            MessageSink
	messageStore           store.MessageStore
	clientConn             WSConn
	applicationId          string
	userId                 string
	messagesAndRouteToSend chan MsgAndRoute
	rawToSend              chan []byte
	routeClosed            chan Route
	notificationsToSend    chan *guble.NotificationMessage
	subscriptions          map[guble.Path]*Route
}

func NewWSHandler(messageSouce PubSubSource, messageSink MessageSink, messageStore store.MessageStore, wsConn WSConn, userId string) *WSHandler {
	server := &WSHandler{
		messageSouce:           messageSouce,
		messageSink:            messageSink,
		messageStore:           messageStore,
		clientConn:             wsConn,
		applicationId:          xid.New().String(),
		userId:                 userId,
		messagesAndRouteToSend: make(chan MsgAndRoute, 100),
		rawToSend:              make(chan []byte, 100),
		notificationsToSend:    make(chan *guble.NotificationMessage, 100),
		routeClosed:            make(chan Route, 100),
		subscriptions:          make(map[guble.Path]*Route),
	}
	return server
}

func (srv *WSHandler) Start() {
	srv.sendConnectionMessage()
	go srv.sendLoop()
	srv.receiveLoop()
}

func (srv *WSHandler) sendLoop() {
	for {
		select {
		case msgAndRoute, ok := <-srv.messagesAndRouteToSend:
			if !ok {
				guble.Info("messageSouce closed the connection -> closing the websocket connection to applicationId=%v", srv.applicationId)
				srv.clientConn.Close()
				return
			}
			if guble.DebugEnabled() {
				guble.Debug("deliver message to applicationId=%v: %v", srv.applicationId, msgAndRoute.Message.MetadataLine())
			}
			if guble.InfoEnabled() {
				guble.Info("sending message: %v", msgAndRoute.Message.MetadataLine())
			}
			if err := srv.clientConn.Send(msgAndRoute.Message.Bytes()); err != nil {
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
		case raw := <-srv.rawToSend:
			if err := srv.clientConn.Send(raw); err != nil {
				guble.Info("applicationId=%v closed the connection", srv.applicationId)
				srv.cleanAndClose()
				break
			}
		case closedRoute := <-srv.routeClosed:
			guble.Info("INFO: router closed route %v -> closing the websocket connection to applicationId=%v", closedRoute, srv.applicationId)
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

		//guble.Debug("websocket_connector, raw message received: %v", string(message))
		cmd, err := guble.ParseCmd(message)
		if err != nil {
			srv.returnError(guble.ERROR_BAD_REQUEST, "error parsing command. %v", err.Error())
			continue
		}
		switch cmd.Name {
		case guble.CMD_SEND:
			srv.handleSend(cmd)
		case guble.CMD_SUBSCRIBE:
			srv.handleSubscribe(cmd)
		case guble.CMD_UNSUBSCRIBE:
			srv.handleUnsubscribe(cmd)
		case guble.CMD_REPLAY:
			srv.handleReplay(cmd)
		default:
			srv.returnError(guble.ERROR_BAD_REQUEST, "unknown command %v", cmd.Name)
		}
	}
}

func (srv *WSHandler) sendConnectionMessage() {
	connected := &guble.NotificationMessage{
		Name: guble.SUCCESS_CONNECTED,
		Arg:  "You are connected to the server.",
		Json: fmt.Sprintf(`{"ApplicationId": "%s", "UserId": "%s", "Time": "%s"}`, srv.applicationId, srv.userId, time.Now().Format(time.RFC3339)),
	}

	if err := srv.clientConn.Send(connected.Bytes()); err != nil {
		guble.Info("applicationId=%v closed the connection", srv.applicationId)
		srv.cleanAndClose()
	}
}

func (srv *WSHandler) handleSend(cmd *guble.Cmd) {
	guble.Debug("sending %v", string(cmd.Bytes()))
	if len(cmd.Arg) == 0 {
		srv.returnError(guble.ERROR_BAD_REQUEST, "send command requires a path argument, but non given", cmd.Name)
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

	srv.returnOK(guble.SUCCESS_SEND, msg.PublisherMessageId)
}

func (srv *WSHandler) handleReplay(cmd *guble.Cmd) {
	if len(cmd.Arg) == 0 {
		srv.returnError(guble.ERROR_BAD_REQUEST, "replay command requires a topic argument, but non given")
		return
	}

	messages := make(chan []byte)
	errors := make(chan error)
	srv.messageStore.Fetch(store.FetchRequest{
		Partition:     cmd.Arg,
		StartId:       0,
		Direction:     1,
		Count:         100,
		MessageC:      messages,
		ErrorCallback: errors,
	})

	go func() {
		guble.Info("start replay")
		for {
			select {
			case msg, open := <-messages:
				if !open {
					guble.Info("replay done")
					return
				}
				guble.Info("replay send %v", string(msg))
				srv.rawToSend <- msg
			case err := <-errors:
				guble.Err("replay error %v", err)
				srv.returnError(guble.ERROR_INTERNAL_SERVER, err.Error())
			}
		}
	}()

}

func (srv *WSHandler) handleSubscribe(cmd *guble.Cmd) {
	if len(cmd.Arg) == 0 {
		srv.returnError(guble.ERROR_BAD_REQUEST, "subscribe command requires a path argument, but non given")
		return
	}
	route := NewRoute(cmd.Arg, srv.messagesAndRouteToSend, srv.routeClosed, srv.applicationId, srv.userId)
	srv.messageSouce.Subscribe(route)
	srv.subscriptions[route.Path] = route
	srv.returnOK(guble.SUCCESS_SUBSCRIBED_TO, cmd.Arg)
}

func (srv *WSHandler) handleUnsubscribe(cmd *guble.Cmd) {
	if len(cmd.Arg) == 0 {
		srv.returnError(guble.ERROR_BAD_REQUEST, "unsubscribe command requires a path argument, but non given")
		return
	}
	route, exist := srv.subscriptions[guble.Path(cmd.Arg)]
	if exist {
		srv.messageSouce.Unsubscribe(route)
		delete(srv.subscriptions, route.Path)
	}
	// even if not subscribed, we return a success here, to be idempotent
	srv.returnOK(guble.SUCCESS_UNSUBSCRIBED_FROM, cmd.Arg)
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
