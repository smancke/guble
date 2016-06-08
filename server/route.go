package server

import (
	"fmt"
	"github.com/smancke/guble/protocol"
)

// NewRoute creates a new route pointer
// 	- `size` is the channel buffer size
func NewRoute(path, applicationID, userID string, size int) *Route {
	return &Route{
		Path:          protocol.Path(path),
		messagesC:     make(chan *protocol.Message, size),
		UserID:        userID,
		ApplicationID: applicationID,
	}
}

// Route represents a topic for subscription that has a channel to receive messages.
type Route struct {
	Path          protocol.Path
	messagesC     chan *protocol.Message
	UserID        string // UserID that subscribed or pushes messages to the router
	ApplicationID string
}

func (r *Route) String() string {
	return fmt.Sprintf("%s:%s:%s", r.Path, r.UserID, r.ApplicationID)
}

func (r *Route) equals(other *Route) bool {
	return r.Path == other.Path &&
		r.UserID == other.UserID &&
		r.ApplicationID == other.ApplicationID
}

// Close closes the route channel.
func (r *Route) Close() {
	close(r.messagesC)
}

// Messages returns the route channel to send or receive messages.
func (r *Route) MessagesChannel() chan *protocol.Message {
	return r.messagesC
}
