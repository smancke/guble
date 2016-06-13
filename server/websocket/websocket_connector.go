package websocket

import (
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server"

	"github.com/gorilla/websocket"
	"github.com/rs/xid"

	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/server/auth"
	"net/http"
	"strings"
	"time"
)

var webSocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSHandler struct {
	Router        server.Router
	prefix        string
	accessManager auth.AccessManager
}

func NewWSHandler(router server.Router, prefix string) (*WSHandler, error) {
	accessManager, err := router.AccessManager()
	if err != nil {
		return nil, err
	}

	return &WSHandler{
		Router:        router,
		prefix:        prefix,
		accessManager: accessManager,
	}, nil
}

func (handle *WSHandler) GetPrefix() string {
	return handle.prefix
}

func (handle *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := webSocketUpgrader.Upgrade(w, r, nil)
	if err != nil {

		logger.WithFields(log.Fields{
			"err": err,
		}).Error("Error on upgrading ")

		return
	}
	defer c.Close()

	NewWebSocket(handle, &wsconn{c}, extractUserID(r.RequestURI)).Start()
}

func (handle *WSHandler) Check() error {
	return nil
}

// WSConnection is a wrapper interface for the needed functions of the websocket.Conn
// It is introduced for testability of the WSHandler
type WSConnection interface {
	Close()
	Send(bytes []byte) (err error)
	Receive(bytes *[]byte) (err error)
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
	return conn.WriteMessage(websocket.BinaryMessage, bytes)
}

func (conn *wsconn) Receive(bytes *[]byte) (err error) {
	_, *bytes, err = conn.ReadMessage()
	return err
}

// WebSocket struct represents a websocket
type WebSocket struct {
	*WSHandler
	WSConnection
	applicationID string
	userID        string
	sendChannel   chan []byte
	receivers     map[protocol.Path]*Receiver
}

func NewWebSocket(handler *WSHandler, wsConn WSConnection, userID string) *WebSocket {
	return &WebSocket{
		WSHandler:     handler,
		WSConnection:  wsConn,
		applicationID: xid.New().String(),
		userID:        userID,
		sendChannel:   make(chan []byte, 10),
		receivers:     make(map[protocol.Path]*Receiver),
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
				//if protocol.DebugEnabled() {
				if len(raw) < 80 {

					logger.WithFields(log.Fields{
						"userId":        ws.userID,
						"applicationID": ws.applicationID,
						"totalSize":     len(raw),
						"actualContent": string(raw),
					}).Debug("Send to client")

				} else {

					logger.WithFields(log.Fields{
						"userId":        ws.userID,
						"applicationID": ws.applicationID,
						"totalSize":     len(raw),
						"actualContent": string(raw[0:79]),
					}).Debug("Send to client")
				}
				//}

				if err := ws.Send(raw); err != nil {

					logger.WithFields(log.Fields{
						"applicationID": ws.applicationID,
					}).Debug("Closed connnection with")

					ws.cleanAndClose()
					break
				}
			}
		}
	}
}

func (ws *WebSocket) checkAccess(raw []byte) bool {
	logger.WithFields(log.Fields{
		"rawMsg": string(raw),
	}).Debug("Raw message")

	if raw[0] == byte('/') {
		path := getPathFromRawMessage(raw)

		logger.WithFields(log.Fields{
			"userID": ws.userID,
			"path":   path,
		}).Debug("Received msg")

		return len(path) == 0 || ws.accessManager.IsAllowed(auth.READ, ws.userID, path)

	}
	return true
}

func getPathFromRawMessage(raw []byte) protocol.Path {
	i := strings.Index(string(raw), ",")
	return protocol.Path(raw[:i])
}

func (ws *WebSocket) receiveLoop() {
	var message []byte
	for {
		err := ws.Receive(&message)
		if err != nil {

			logger.WithFields(log.Fields{
				"applicationID": ws.applicationID,
			}).Debug("Closed connnection by application")

			ws.cleanAndClose()
			break
		}

		//protocol.Debug("websocket_connector, raw message received: %v", string(message))
		cmd, err := protocol.ParseCmd(message)
		if err != nil {
			ws.sendError(protocol.ERROR_BAD_REQUEST, "error parsing command. %v", err.Error())
			continue
		}
		switch cmd.Name {
		case protocol.CmdSend:
			ws.handleSendCmd(cmd)
		case protocol.CmdReceive:
			ws.handleReceiveCmd(cmd)
		case protocol.CmdCancel:
			ws.handleCancelCmd(cmd)
		default:
			ws.sendError(protocol.ERROR_BAD_REQUEST, "unknown command %v", cmd.Name)
		}
	}
}

func (ws *WebSocket) sendConnectionMessage() {
	n := &protocol.NotificationMessage{
		Name: protocol.SUCCESS_CONNECTED,
		Arg:  "You are connected to the server.",
		Json: fmt.Sprintf(`{"ApplicationId": "%s", "UserId": "%s", "Time": "%s"}`, ws.applicationID, ws.userID, time.Now().Format(time.RFC3339)),
	}
	ws.sendChannel <- n.Bytes()
}

func (ws *WebSocket) handleReceiveCmd(cmd *protocol.Cmd) {
	rec, err := NewReceiverFromCmd(
		ws.applicationID,
		cmd,
		ws.sendChannel,
		ws.Router,
		ws.userID,
	)
	if err != nil {

		logger.WithFields(log.Fields{
			"err": err,
		}).Error("Client error in handleReceiveCmd")

		ws.sendError(protocol.ERROR_BAD_REQUEST, err.Error())
		return
	}
	ws.receivers[rec.path] = rec
	rec.Start()
}

func (ws *WebSocket) handleCancelCmd(cmd *protocol.Cmd) {
	if len(cmd.Arg) == 0 {
		ws.sendError(protocol.ERROR_BAD_REQUEST, "- command requires a path argument, but none given")
		return
	}
	path := protocol.Path(cmd.Arg)
	rec, exist := ws.receivers[path]
	if exist {
		rec.Stop()
		delete(ws.receivers, path)
	}
}

func (ws *WebSocket) handleSendCmd(cmd *protocol.Cmd) {
	logger.WithFields(log.Fields{
		"cmd": string(cmd.Bytes()),
	}).Debug("Sending ")

	if len(cmd.Arg) == 0 {
		ws.sendError(protocol.ERROR_BAD_REQUEST, "send command requires a path argument, but none given")
		return
	}

	args := strings.SplitN(cmd.Arg, " ", 2)
	msg := &protocol.Message{
		Path:          protocol.Path(args[0]),
		ApplicationID: ws.applicationID,
		UserID:        ws.userID,
		HeaderJSON:    cmd.HeaderJSON,
		Body:          cmd.Body,
	}
	if len(args) == 2 {
		msg.MessageID = args[1]
	}

	ws.Router.HandleMessage(msg)

	ws.sendOK(protocol.SUCCESS_SEND, msg.MessageID)
}

func (ws *WebSocket) cleanAndClose() {

	logger.WithFields(log.Fields{
		"applicationID": ws.applicationID,
	}).Debug("Closing applicationId")

	for path, rec := range ws.receivers {
		rec.Stop()
		delete(ws.receivers, path)
	}

	ws.Close()
}

func (ws *WebSocket) sendError(name string, argPattern string, params ...interface{}) {
	n := &protocol.NotificationMessage{
		Name:    name,
		Arg:     fmt.Sprintf(argPattern, params...),
		IsError: true,
	}
	ws.sendChannel <- n.Bytes()
}

func (ws *WebSocket) sendOK(name string, argPattern string, params ...interface{}) {
	n := &protocol.NotificationMessage{
		Name:    name,
		Arg:     fmt.Sprintf(argPattern, params...),
		IsError: false,
	}
	ws.sendChannel <- n.Bytes()
}

// Extracts the userID out of an URI or empty string if format not met
// Example:
// 		http://example.com/user/user01/ -> user01
// 		http://example.com/user/ -> ""
func extractUserID(uri string) string {
	uriParts := strings.SplitN(uri, "/user/", 2)
	if len(uriParts) != 2 {
		return ""
	}
	return uriParts[1]
}
