package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/smancke/guble/protocol"
)

type RouteOptions struct {
	Path protocol.Path

	ChannelSize int

	// QueueSize specify the size of the internal queue slice.
	// How many items to hold before the channel is closed.
	// If set to `0` then the queue will have no capacity and the messages are sent
	// directly, without buffering
	QueueSize int

	// Timeout to define how long to wait for the message to be read on the channel
	// if timeout is reached the route is closed
	Timeout time.Duration

	RouteParams
}

type RouteParams map[string]string

func (rp *RouteParams) String() string {
	s := make([]string, 0, len(*rp))
	for k, v := range *rp {
		s = append(s, fmt.Sprintf("%s: %s", k, v))
	}
	return strings.Join(s, " ")
}

// Equal verifies if the `receiver` params are the same as `other` params
// the `keys` param specifies which keys to check in case the match has to be
// done only on a separate set of keys and not all
func (rp *RouteParams) Equal(other RouteParams, keys ...string) bool {
	if len(keys) > 0 {
		return rp.partialEqual(other, keys)

	}

	if len(*rp) != len(other) {
		return false
	}

	for k, v := range *rp {
		if v2, ok := other[k]; !ok {
			return false
		} else if v != v2 {
			return false
		}
	}

	return true
}

func (rp *RouteParams) partialEqual(other RouteParams, fields []string) bool {
	for _, key := range fields {
		if v, ok := other[key]; !ok {
			return false
		} else if v != (*rp)[key] {
			return false
		}
	}

	return true
}

func (rp *RouteParams) Get(key string) string {
	return (*rp)[key]
}

func (rp *RouteParams) Set(key, value string) {
	(*rp)[key] = value
}
