package client

import (
	"fmt"
	"github.com/smancke/guble/guble"
	"golang.org/x/net/websocket"
)

type Client struct {
	ws             *websocket.Conn
	messages       chan *guble.Message
	statusMessages chan *guble.NotificationMessage
	errors         chan *guble.NotificationMessage
}

func Open(url, origin string, channelSize int) (*Client, error) {
	ws, err := websocket.Dial(url, "", origin)
	if err != nil {
		return nil, err
	}

	c := &Client{
		ws:             ws,
		messages:       make(chan *guble.Message, channelSize),
		statusMessages: make(chan *guble.NotificationMessage, channelSize),
		errors:         make(chan *guble.NotificationMessage, channelSize),
	}
	go c.readLoop()
	return c, nil
}

func (c *Client) readLoop() {
	var msg = make([]byte, 512)
	var n int
	var err error
	var parsed interface{}
	shouldStop := false
	for !shouldStop {
		func() {
			defer guble.PanicLogger()

			if n, err = c.ws.Read(msg); err != nil {
				guble.Err("%#v", err)
				c.errors <- clientErrorMessage(err.Error())
				shouldStop = true
				return
			}
			guble.Debug("client raw read> %s", msg[:n])

			parsed, err = guble.ParseMessage(msg[:n])
			if err != nil {
				guble.Err("parsing message failed %v", err)
				c.errors <- clientErrorMessage(err.Error())
				return
			}

			switch message := parsed.(type) {
			case *guble.Message:
				c.messages <- message
			case *guble.NotificationMessage:
				if message.IsError {
					c.errors <- message
				} else {
					c.statusMessages <- message
				}
			}
		}()
	}
}

func (c *Client) Subscribe(path string) error {
	_, err := c.ws.Write(bytef("subscribe %v", path))
	return err
}

func (c *Client) Send(path string, body string) error {
	return c.SendBytes(path, []byte(body))
}

func (c *Client) SendBytes(path string, body []byte) error {
	cmd := &guble.Cmd{
		Name: guble.CMD_SEND,
		Arg:  path,
		Body: body,
	}

	return c.WriteRawMessage(cmd.Bytes())
}

func (c *Client) WriteRawMessage(message []byte) error {
	_, err := c.ws.Write(message)
	return err
}

func (c *Client) Messages() chan *guble.Message {
	return c.messages
}

func (c *Client) StatusMessages() chan *guble.NotificationMessage {
	return c.statusMessages
}

func (c *Client) Errors() chan *guble.NotificationMessage {
	return c.errors
}

func (c *Client) Close() {
	c.ws.Close()
}

func clientErrorMessage(message string) *guble.NotificationMessage {
	return &guble.NotificationMessage{
		IsError: true,
		Name:    "clientError",
		Arg:     message,
	}
}

func bytef(pattern string, args ...interface{}) []byte {
	return []byte(fmt.Sprintf(pattern, args...))
}
