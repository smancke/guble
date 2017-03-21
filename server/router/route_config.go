package router

import (
	"time"

	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/store"
)

// Matcher is a func type that receives two route configurations pointers as parameters and
// returns true if the routes are matching
type Matcher func(RouteConfig, RouteConfig, ...string) bool

type RouteConfig struct {
	RouteParams

	Path protocol.Path

	ChannelSize int

	// QueueSize specifies the size of the internal queue slice
	// (how many items to hold before the channel is closed).
	// If set to `0` then the queue will have no capacity and the messages
	// are directly sent, without buffering.
	QueueSize int

	// Timeout defines how long to wait for the message to be read on the channel.
	// If Timeout is reached the route is closed.
	Timeout time.Duration

	// Matcher if set will be used to check equality of the routes
	Matcher Matcher `json:"-"`

	// FetchRequest to fetch messages before subscribing
	// The Partition field of the FetchRequest is overrided with the Partition of the Route topic
	FetchRequest *store.FetchRequest `json:"-"`
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

	return rc.Filter(m.Filters)
}

// Filter returns true if all filters are matched on the route
func (rc *RouteConfig) Filter(filters map[string]string) bool {
	for key, value := range filters {
		if rc.Get(key) != value {
			return false
		}
	}

	return true
}
