package client

import (
	"github.com/smancke/guble/guble"
	"golang.org/x/net/websocket"
)

type Client struct {
	ws            *websocket.Conn
	Messages      chan guble.Message
	Notifications chan string
	StatusError   chan string
}

func Open(url, origin string) (*Client, error) {
	ws, err := websocket.Dial(url, "", origin)
	if err != nil {
		return nil, err
	}
	return &Client{ws}, nil
}

func (c *Client) close() {
	c.ws.Close()
}
