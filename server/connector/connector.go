package connector

import (
	"net/http"

	"github.com/docker/distribution/health"
	"github.com/smancke/guble/protocol"
)

type Request interface {
	Subscriber() Subscriber
	Message() *protocol.Message
}

type Sender interface {
	Send(Request) error
}

type Connector interface {
	health.Checker
	SubscriptionManager

	Start() error
	Stop() error

	Prefix() string
	http.Handler
}
