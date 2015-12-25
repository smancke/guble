package client

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/smancke/guble/guble"
	"net/http"
	"time"
)

type Client struct {
	ws             *websocket.Conn
	messages       chan *guble.Message
	statusMessages chan *guble.NotificationMessage
	errors         chan *guble.NotificationMessage
	url            string
	origin         string
	shouldStop     chan bool
	autoReconnect  bool
}

func Open(url, origin string, channelSize int, autoReconnect bool) (*Client, error) {
	c := &Client{
		messages:       make(chan *guble.Message, channelSize),
		statusMessages: make(chan *guble.NotificationMessage, channelSize),
		errors:         make(chan *guble.NotificationMessage, channelSize),
		url:            url,
		origin:         origin,
		shouldStop:     make(chan bool, 1),
		autoReconnect:  autoReconnect,
	}

	if err := c.connect(); err != nil {
		return c, err
	}
	if !autoReconnect {
		go c.readLoop()
	} else {
		go func() {
			for {
				if c.ws != nil {
					c.readLoop()
				}

				select {
				case <-c.shouldStop:
					break
				default:
				}

				if err := c.connect(); err != nil {
					guble.Err("error on connect, retry in 1 second")
					time.Sleep(time.Second * 1)
				} else {
					guble.Err("connected again")
				}
			}
		}()

	}
	return c, nil
}

func (c *Client) connect() error {
	guble.Info("connecting to %v", c.url)
	var err error
	header := http.Header{"Origin": []string{c.origin}}
	c.ws, _, err = websocket.DefaultDialer.Dial(c.url, header)
	//c.ws, err = websocket.Dial(c.url, "", c.origin)
	if err != nil {
		return err
	}
	guble.Info("connected to %v", c.url)
	return nil
}

func (c *Client) readLoop() {
	var err error
	var parsed interface{}
	connectionError := false
	for !connectionError {
		func() {
			defer guble.PanicLogger()
			var msg []byte
			if _, msg, err = c.ws.ReadMessage(); err != nil {
				select {
				case <-c.shouldStop:
					c.shouldStop <- true
					return
				default:
					guble.Err("%#v", err.Error())
					c.errors <- clientErrorMessage(err.Error())
					connectionError = true
					return
				}
			}
			guble.Debug("client raw read> %s", msg)

			parsed, err = guble.ParseMessage(msg)
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
	cmd := &guble.Cmd{
		Name: guble.CMD_SUBSCRIBE,
		Arg:  path,
	}
	err := c.ws.WriteMessage(websocket.BinaryMessage, cmd.Bytes())
	return err
}

func (c *Client) Send(path string, body string, header string) error {
	return c.SendBytes(path, []byte(body), header)
}

func (c *Client) SendBytes(path string, body []byte, header string) error {
	cmd := &guble.Cmd{
		Name:       guble.CMD_SEND,
		Arg:        path,
		Body:       body,
		HeaderJson: header,
	}

	return c.WriteRawMessage(cmd.Bytes())
}

func (c *Client) WriteRawMessage(message []byte) error {
	return c.ws.WriteMessage(websocket.BinaryMessage, message)
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
	c.shouldStop <- true
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
