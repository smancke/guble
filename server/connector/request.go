package connector

import "github.com/cosminrentea/gobbler/protocol"

type Request interface {
	Subscriber() Subscriber
	Message() *protocol.Message
}

type request struct {
	subscriber Subscriber
	message    *protocol.Message
}

func NewRequest(s Subscriber, m *protocol.Message) Request {
	return &request{s, m}
}

func (r *request) Subscriber() Subscriber {
	return r.subscriber
}

func (r *request) Message() *protocol.Message {
	return r.message
}
