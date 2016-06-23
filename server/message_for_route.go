package server

import (
	"github.com/smancke/guble/protocol"

	"fmt"
)

// MessageForRoute is a wrapper that aggregates the message and the route.
// It is useful for sending both pieces of information over a channel.
type MessageForRoute struct {
	Message *protocol.Message
	Route   *Route
}

func (mfr *MessageForRoute) String() string {
	return fmt.Sprintf("Message %s for route %s", mfr.Message, mfr.Route)
}
