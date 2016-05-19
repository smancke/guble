package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"

	"github.com/gorilla/websocket"
	"github.com/rs/xid"

	"fmt"
	"github.com/smancke/guble/server/auth"
	"net/http"
	"strings"
	"time"
)

var webSocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSHandler struct {
	Router        PubSubSource
	MessageSink   MessageSink
	prefix        string
	messageStore  store.MessageStore
	accessManager auth.AccessManager
}

func NewWSHandler(prefix string) *WSHandler {
	return &WSHandler{prefix: prefix}
}

func (handle *WSHandler) GetPrefix() string {
	return handle.prefix
}

func (handle *WSHandler) SetMessageEntry(messageSink MessageSink) {
	handle.MessageSink = messageSink
}

func (handle *WSHandler) SetRouter(router PubSubSource) {
	handle.Router = router
}

func (entry *WSHandler) SetMessageStore(messageStore store.MessageStore) {
	entry.messageStore = messageStore
}

func (entry *WSHandler) SetAccessManager(accessManager auth.AccessManager) {
	entry.accessManager = accessManager
}

func (handle *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := webSocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		guble.Warn("error on upgrading %v", err.Error())
		return
	}
	defer c.Close()

	NewWebSocket(handle, &wsconn{c}, extractUserId(r.RequestURI)).Start()
}

// wsconnImpl is a Wrapper of the websocket.Conn
// implementing the interface WSConn for better testability
type wsconn struct {
	*websocket.Conn
}

func (conn *wsconn) Close() {
	conn.Close()
}

func (conn *wsconn) Send(bytes []byte) (err error) {
	return conn.WriteMessage(websocket.BinaryMessage, bytes)
}

func (conn *wsconn) Receive(bytes *[]byte) (err error) {
	_, *bytes, err = conn.ReadMessage()
	return err
}

// Represents a websocket
type WebSocket struct {
	*WSHandler
	WSConnection
	applicationId string
	userId        string
	sendChannel   chan []byte
	receivers     map[guble.Path]*Receiver
}

func NewWebSocket(handler *WSHandler, wsConn WSConnection, userId string) *WebSocket {
	return &WebSocket{
		WSHandler:     handler,
		WSConnection:  wsConn,
		applicationId: xid.New().String(),
		userId:        userId,
		sendChannel:   make(chan []byte, 10),
		receivers:     make(map[guble.Path]*Receiver),
	}
}

func (ws *WebSocket) Start() error {
	ws.sendConnectionMessage()
	go ws.sendLoop()
	ws.receiveLoop()
	return nil
}

func (ws *WebSocket) sendLoop() {
	for {
		select {
		case raw := <-ws.sendChannel:

			if ws.checkAccess(raw) {
				if guble.DebugEnabled() {
					if len(raw) < 80 {
						guble.Debug("send to client (userId=%v, applicationId=%v, totalSize=%v): %v", ws.userId, ws.applicationId, len(raw), string(raw))
					} else {
						guble.Debug("send to client (userId=%v, applicationId=%v, totalSize=%v): %v...", ws.userId, ws.applicationId, len(raw), string(raw[0:79]))
					}
				}

				if err := ws.Send(raw); err != nil {
					guble.Info("applicationId=%v closed the connection", ws.applicationId)
					ws.cleanAndClose()
					break
				}
			}
		}
	}
}

func (ws *WebSocket) checkAccess(raw []byte) bool {
	guble.Debug("raw message: %v", string(raw))
	if raw[0] == byte('/') {
		path := getPathFromRawMessage(raw)
		guble.Debug("Received msg %v %v", ws.userId, path)
		return len(path) == 0 || ws.accessManager.IsAllowed(auth.READ, ws.userId, path)

	}
	return true
}

func getPathFromRawMessage(raw []byte) guble.Path {
	i := strings.Index(string(raw), ",")
	return guble.Path(raw[:i])
}

func (ws *WebSocket) receiveLoop() {
	var message []byte
	for {
		err := ws.Receive(&message)
		if err != nil {
			guble.Info("applicationId=%v closed the connection", ws.applicationId)
			ws.cleanAndClose()
			break
		}

		//guble.Debug("websocket_connector, raw message received: %v", string(message))
		cmd, err := guble.ParseCmd(message)
		if err != nil {
			ws.sendError(guble.ERROR_BAD_REQUEST, "error parsing command. %v", err.Error())
			continue
		}
		switch cmd.Name {
		case guble.CmdSend:
			ws.handleSendCmd(cmd)
		case guble.CmdReceive:
			ws.handleReceiveCmd(cmd)
		case guble.CmdCancel:
			ws.handleCancelCmd(cmd)
		default:
			ws.sendError(guble.ERROR_BAD_REQUEST, "unknown command %v", cmd.Name)
		}
	}
}

func (ws *WebSocket) sendConnectionMessage() {
	n := &guble.NotificationMessage{
		Name: guble.SUCCESS_CONNECTED,
		Arg:  "You are connected to the server.",
		Json: fmt.Sprintf(`{"ApplicationId": "%s", "UserId": "%s", "Time": "%s"}`, ws.applicationId, ws.userId, time.Now().Format(time.RFC3339)),
	}
	ws.sendChannel <- n.Bytes()
}

func (ws *WebSocket) handleReceiveCmd(cmd *guble.Cmd) {
	rec, err := NewReceiverFromCmd(
		ws.applicationId,
		cmd,
		ws.sendChannel,
		ws.Router,
		ws.messageStore,
		ws.userId,
	)
	if err != nil {
		guble.Info("client error in handleReceiveCmd: %v", err.Error())
		ws.sendError(guble.ERROR_BAD_REQUEST, err.Error())
		return
	}
	ws.receivers[rec.path] = rec
	rec.Start()
}

func (ws *WebSocket) handleCancelCmd(cmd *guble.Cmd) {
	if len(cmd.Arg) == 0 {
		ws.sendError(guble.ERROR_BAD_REQUEST, "- command requires a path argument, but non given")
		return
	}
	path := guble.Path(cmd.Arg)
	rec, exist := ws.receivers[path]
	if exist {
		rec.Stop()
		delete(ws.receivers, path)
	}
}

func (ws *WebSocket) handleSendCmd(cmd *guble.Cmd) {
	guble.Debug("sending %v", string(cmd.Bytes()))
	if len(cmd.Arg) == 0 {
		ws.sendError(guble.ERROR_BAD_REQUEST, "send command requires a path argument, but non given")
		return
	}

	args := strings.SplitN(cmd.Arg, " ", 2)
	msg := &guble.Message{
		Path: guble.Path(args[0]),
		PublisherApplicationId: ws.applicationId,
		PublisherUserId:        ws.userId,
		HeaderJSON:             cmd.HeaderJSON,
		Body:                   cmd.Body,
	}
	if len(args) == 2 {
		msg.PublisherMessageId = args[1]
	}

	ws.MessageSink.HandleMessage(msg)

	ws.sendOK(guble.SUCCESS_SEND, msg.PublisherMessageId)
}

func (ws *WebSocket) cleanAndClose() {
	guble.Info("closing applicationId=%v", ws.applicationId)

	for path, rec := range ws.receivers {
		rec.Stop()
		delete(ws.receivers, path)
	}

	ws.Close()
}

func (ws *WebSocket) sendError(name string, argPattern string, params ...interface{}) {
	n := &guble.NotificationMessage{
		Name:    name,
		Arg:     fmt.Sprintf(argPattern, params...),
		IsError: true,
	}
	ws.sendChannel <- n.Bytes()
}

func (ws *WebSocket) sendOK(name string, argPattern string, params ...interface{}) {
	n := &guble.NotificationMessage{
		Name:    name,
		Arg:     fmt.Sprintf(argPattern, params...),
		IsError: false,
	}
	ws.sendChannel <- n.Bytes()
}
