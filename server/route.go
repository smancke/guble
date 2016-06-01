package server

import (
	"fmt"
	"github.com/smancke/guble/protocol"
)

// NewRoute creates a new route pointer
func NewRoute(path, applicationID, userID string, channel chan *MessageForRoute) *Route {
	return &Route{
		Path:          protocol.Path(path),
		messagesC:     channel,
		UserID:        userID,
		ApplicationID: applicationID,
	}
}

// Route represents a topic for subscription that has a channel to receive message
type Route struct {
	Path          protocol.Path
	messagesC     chan *MessageForRoute
	UserID        string // UserID that subscribed or pushes messages to the router
	ApplicationID string // ApplicationID that
}

func (r *Route) String() string {
	return fmt.Sprintf("%s:%s:%s", r.Path, r.UserID, r.ApplicationID)
}

func (r *Route) equals(other *Route) bool {
	return r.Path == other.Path &&
		r.UserID == other.UserID &&
		r.ApplicationID == other.ApplicationID
}

// Close closes the route channel
func (r *Route) Close() {
	close(r.messagesC)
}

// Messages return the route channel to send or receive messages
func (r *Route) Messages() chan *MessageForRoute {
	return r.messagesC
}

// MsgAndRoute is a wrapper that provides the message and the route togheter.
// Useful for sending both information over a channel
type MessageForRoute struct {
	Message *protocol.Message
	Route   *Route
}

func (m *MessageForRoute) String() string {
	return fmt.Sprintf("Message %s for route %s")
}
