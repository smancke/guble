package router

import (
	"time"

	"github.com/smancke/guble/protocol"
)

// Matcher is a func type that receives two route configurations pointers as parameters and
// returns true if the routes are matching
type Matcher func(RouteConfig, RouteConfig, ...string) bool

type RouteConfig struct {
	RouteParams

	Path protocol.Path

	ChannelSize int

	// queueSize specifies the size of the internal queue slice
	// (how many items to hold before the channel is closed).
	// If set to `0` then the queue will have no capacity and the messages
	// are directly sent, without buffering.
	queueSize int

	// timeout defines how long to wait for the message to be read on the channel.
	// If timeout is reached the route is closed.
	timeout time.Duration

	// If Matcher is set will be used to check equality of the routes
	Matcher Matcher
}

func (rc *RouteConfig) Equal(other RouteConfig, keys ...string) bool {
	if rc.Matcher != nil {
		return rc.Matcher(*rc, other, keys...)
	}
	return rc.Path == other.Path && rc.RouteParams.Equal(other.RouteParams, keys...)
}

// messageFilter returns true if the route matches message filters
func (rc *RouteConfig) messageFilter(m *protocol.Message) bool {
	if m.Filters == nil {
		return true
	}

	for key, value := range m.Filters {
		if rc.Get(key) != value {
			return false
		}
	}

	return true
}
