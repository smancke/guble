package client

import (
	"github.com/gorilla/websocket"
	"github.com/smancke/guble/protocol"
	"net/http"
	"time"
)

type WSConnection interface {
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
}

func DefaultConnectionFactory(url string, origin string) (WSConnection, error) {
	protocol.Info("connecting to %v", url)
	header := http.Header{"Origin": []string{origin}}
	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		return nil, err
	}
	protocol.Info("connected to %v", url)
	return conn, nil
}

type WSConnectionFactory func(url string, origin string) (WSConnection, error)

type Client interface {
	Start() error
	Close()

	Subscribe(path string) error
	Unsubscribe(path string) error

	Send(path string, body string, header string) error
	SendBytes(path string, body []byte, header string) error

	WriteRawMessage(message []byte) error
	Messages() chan *protocol.Message
	StatusMessages() chan *protocol.NotificationMessage
	Errors() chan *protocol.NotificationMessage

	SetWSConnectionFactory(WSConnectionFactory)
	IsConnected() bool
}

type client struct {
	ws                  WSConnection
	messages            chan *protocol.Message
	statusMessages      chan *protocol.NotificationMessage
	errors              chan *protocol.NotificationMessage
	url                 string
	origin              string
	shouldStopChan      chan bool
	shouldStopFlag      bool
	autoReconnect       bool
	wSConnectionFactory func(url string, origin string) (WSConnection, error)
	// flag, to indicate if the client is connected
	connected bool
}

// shortcut for New() and Start()
func Open(url, origin string, channelSize int, autoReconnect bool) (Client, error) {
	c := New(url, origin, channelSize, autoReconnect)
	c.SetWSConnectionFactory(DefaultConnectionFactory)
	return c, c.Start()
}

// Construct a new client, without starting the connection
func New(url, origin string, channelSize int, autoReconnect bool) Client {
	return &client{
		messages:       make(chan *protocol.Message, channelSize),
		statusMessages: make(chan *protocol.NotificationMessage, channelSize),
		errors:         make(chan *protocol.NotificationMessage, channelSize),
		url:            url,
		origin:         origin,
		shouldStopChan: make(chan bool, 1),
		autoReconnect:  autoReconnect,
	}
}

func (c *client) SetWSConnectionFactory(connection WSConnectionFactory) {
	c.wSConnectionFactory = connection
}

func (c *client) IsConnected() bool {
	return c.connected
}

// Connect and start the read go routine.
// If an error occurs on first connect, it will be returned.
// Further connection errors will only be logged.
func (c *client) Start() error {
	var err error
	c.ws, err = c.wSConnectionFactory(c.url, c.origin)
	c.connected = err == nil

	if c.autoReconnect {
		go c.startWithReconnect()
	} else {
		if c.IsConnected() {
			go c.readLoop()
		}
	}
	return err
}

func (c *client) startWithReconnect() {
	for {
		if c.IsConnected() {
			err := c.readLoop()
			if err == nil {
				return
			}
		}

		if c.shouldStop() {

			return
		}

		var err error
		c.ws, err = c.wSConnectionFactory(c.url, c.origin)
		if err != nil {
			c.connected = false
			protocol.Err("error on connect, retry in 50ms: %v", err)
			time.Sleep(time.Millisecond * 50)
		} else {
			c.connected = true
			protocol.Err("connected again")
		}
	}
}

func (c *client) readLoop() error {
	for {
		if _, msg, err := c.ws.ReadMessage(); err != nil {
			c.connected = false
			if c.shouldStop() {
				return nil
			} else {
				protocol.Err("read error: %v", err.Error())
				c.errors <- clientErrorMessage(err.Error())
				return err
			}
		} else {
			protocol.Debug("raw> %s", msg)
			c.handleIncommoingMessage(msg)
		}
	}
}

func (c *client) shouldStop() bool {
	if c.shouldStopFlag {
		return true
	}
	select {
	case <-c.shouldStopChan:
		c.shouldStopFlag = true
		return true
	default:
		return false
	}
}

func (c *client) handleIncommoingMessage(msg []byte) {
	parsed, err := protocol.ParseMessage(msg)
	if err != nil {
		protocol.Err("parsing message failed %v", err)
		c.errors <- clientErrorMessage(err.Error())
		return
	}

	switch message := parsed.(type) {
	case *protocol.Message:
		c.messages <- message
	case *protocol.NotificationMessage:
		if message.IsError {
			select {
			case c.errors <- message:
			default:
			}
		} else {
			select {
			case c.statusMessages <- message:
			default:
			}
		}
	}
}

func (c *client) Subscribe(path string) error {
	cmd := &protocol.Cmd{
		Name: protocol.CmdReceive,
		Arg:  path,
	}
	err := c.ws.WriteMessage(websocket.BinaryMessage, cmd.Bytes())
	return err
}

func (c *client) Unsubscribe(path string) error {
	cmd := &protocol.Cmd{
		Name: protocol.CmdCancel,
		Arg:  path,
	}
	err := c.ws.WriteMessage(websocket.BinaryMessage, cmd.Bytes())
	return err
}

func (c *client) Send(path string, body string, header string) error {
	return c.SendBytes(path, []byte(body), header)
}

func (c *client) SendBytes(path string, body []byte, header string) error {
	cmd := &protocol.Cmd{
		Name:       protocol.CmdSend,
		Arg:        path,
		Body:       body,
		HeaderJSON: header,
	}

	return c.WriteRawMessage(cmd.Bytes())
}

func (c *client) WriteRawMessage(message []byte) error {
	return c.ws.WriteMessage(websocket.BinaryMessage, message)
}

func (c *client) Messages() chan *protocol.Message {
	return c.messages
}

func (c *client) StatusMessages() chan *protocol.NotificationMessage {
	return c.statusMessages
}

func (c *client) Errors() chan *protocol.NotificationMessage {
	return c.errors
}

func (c *client) Close() {
	c.shouldStopChan <- true
	c.ws.Close()
}

func clientErrorMessage(message string) *protocol.NotificationMessage {
	return &protocol.NotificationMessage{
		IsError: true,
		Name:    "clientError",
		Arg:     message,
	}
}
