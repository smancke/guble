package connector

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/router"
	"github.com/smancke/guble/server/store"
)

var (
	ErrSubscriberExists       = errors.New("Subscriber exists.")
	ErrSubscriberDoesntExists = errors.New("Subscribers doesn't exist.")

	ErrRouteChannelClosed = errors.New("Subscriber route channel has been closed.")
)

type Subscriber interface {
	Key() string
	Route() *router.Route
	Loop(context.Context, chan *protocol.Message) error
}

type subscriberData struct {
	Topic  protocol.Path
	Params router.RouteParams
	LastID uint64
}

type subscriber struct {
	params router.RouteParams
	route  *router.Route
}

func NewSubscriber(topic protocol.Path, params router.RouteParams, fetchRequest *store.FetchRequest) Subscriber {
	return &subscriber{
		params,
		router.NewRoute(router.RouteConfig{
			Path:         topic,
			RouteParams:  params,
			FetchRequest: fetchRequest,
		}),
	}
}

func NewSubscriberFromJSON(data []byte) (Subscriber, error) {
	sData := subscriberData{}
	err := json.Unmarshal(data, &sData)
	if err != nil {
		return nil, err
	}

	var fr *store.FetchRequest
	if sData.LastID > 0 {
		fr = store.NewFetchRequest(sData.Topic.Partition(), sData.LastID, 0, store.DirectionForward, -1)
	}

	return NewSubscriber(sData.Topic, sData.Params, fr), nil
}

// TODO Bogdan Implemenet unique key generation from params
func (s *subscriber) Key() string {
	return "DUMMY KEY"
}

func (s *subscriber) Route() *router.Route {
	return s.route
}

func (s *subscriber) Loop(ctx context.Context, pipeline chan *protocol.Message) error {
	var (
		opened bool = true
		m      *protocol.Message
	)
	for opened {
		select {
		case m, opened = <-s.route.MessagesChannel():
			pipeline <- m
		case <-ctx.Done():
			return nil
		}
	}
	return ErrRouteChannelClosed
}
