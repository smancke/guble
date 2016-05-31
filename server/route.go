package server

import (
	"github.com/smancke/guble/protocol"
)

// MsgAndRoute is a wrapper that provides the message and the route togheter.
// Useful for sending both information over a channel
type MsgAndRoute struct {
	Message *protocol.Message
	Route   *Route
}

// Route represents a topic for subscription that has a channel to receive message
type Route struct {
	Path          protocol.Path
	C             chan MsgAndRoute
	UserID        string // UserID that subscribed or pushes messages to the router
	ApplicationID string // ApplicationID that
}

// NewRoute creates a new route pointer
func NewRoute(
	path string,
	channel chan MsgAndRoute,
	applicationID string,
	userID string) *Route {
	return &Route{
		Path:          protocol.Path(path),
		C:             channel,
		UserID:        userID,
		ApplicationID: applicationID,
	}
}

func (r *Route) equals(other *Route) bool {
	return r.Path == other.Path &&
		r.UserID == other.UserID &&
		r.ApplicationID == other.ApplicationID
}
