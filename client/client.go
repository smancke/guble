package client

import (
	"bufio"
	"github.com/smancke/guble/guble"
	"golang.org/x/net/websocket"
	"os"
)

type Client struct {
	ws             *websocket.Conn
	messages       chan *guble.Message
	statusMessages chan string
	errors         chan string
}

func Open(url, origin string, channelSize int) (*Client, error) {
	ws, err := websocket.Dial(url, "", origin)
	if err != nil {
		return nil, err
	}

	c := &Client{
		ws:             ws,
		messages:       make(chan *guble.Message, channelSize),
		statusMessages: make(chan string, channelSize),
		errors:         make(chan string, channelSize),
	}
	go readLoop(c)
	go writeLoop(c)
	return c, nil
}

func readLoop(c *Client) {
	var msg = make([]byte, 512)
	var n int
	var err error
	var message *guble.Message
	for {
		// TODO: catch panics
		if n, err = c.ws.Read(msg); err != nil {
			guble.Err("%v", err)
			c.errors <- err.Error()
			continue
		}
		guble.Debug("%s", msg[:n])
		message, err = guble.ParseMessage(msg[:n])
		if err != nil {
			guble.Err("parsing message failed %v", err)
			c.errors <- err.Error()
			continue
		}
		c.messages <- message
	}
}

func writeLoop(c *Client) {
	// TODO: Implement proper write logic
	for {
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		if _, err := c.ws.Write([]byte(text)); err != nil {
			guble.Err(err.Error())
		}
	}
}

func (c *Client) Messages() chan *guble.Message {
	return c.messages
}

func (c *Client) StatusMessages() chan string {
	return c.statusMessages
}

func (c *Client) Errors() chan string {
	return c.errors
}

func (c *Client) close() {
	c.ws.Close()
}
