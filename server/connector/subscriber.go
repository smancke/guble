package connector

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/router"
	"github.com/smancke/guble/server/store"
)

var (
	ErrSubscriberExists       = errors.New("Subscriber exists.")
	ErrSubscriberDoesNotExist = errors.New("Subscriber does not exist.")

	ErrRouteChannelClosed = errors.New("Subscriber route channel has been closed.")
)

type Subscriber interface {
	Key() string
	Route() *router.Route
	Loop(context.Context, Queue) error
	SetLastID(ID uint64) error
	Cancel()
	Encode() ([]byte, error)
}

type subscriberData struct {
	Topic  protocol.Path
	Params router.RouteParams
	LastID uint64
}

type subscriber struct {
	data subscriberData

	route  *router.Route
	key    string
	cancel context.CancelFunc
}

func NewSubscriber(topic protocol.Path, params router.RouteParams, fetchRequest *store.FetchRequest) Subscriber {
	data := subscriberData{
		Topic:  topic,
		Params: params,
	}
	return &subscriber{
		data: data,
		route: router.NewRoute(router.RouteConfig{
			Path:         topic,
			RouteParams:  params,
			FetchRequest: fetchRequest,
		}),
	}
}

func NewSubscriberFromJSON(data []byte) (Subscriber, error) {
	sd := subscriberData{}
	err := json.Unmarshal(data, &sd)
	if err != nil {
		return nil, err
	}

	var fr *store.FetchRequest
	if sd.LastID > 0 {
		fr = store.NewFetchRequest(sd.Topic.Partition(), sd.LastID, 0, store.DirectionForward, -1)
	}

	return NewSubscriber(sd.Topic, sd.Params, fr), nil
}

func (s *subscriber) String() string {
	return s.Key()
}

// TODO Bogdan extract the generation of the key as an external method to be reused
func (s *subscriber) Key() string {
	if s.key == "" {
		s.key = GenerateKey(string(s.data.Topic), s.data.Params)
	}
	return s.key
}

func (s *subscriber) Route() *router.Route {
	return s.route
}

func (s *subscriber) Loop(ctx context.Context, q Queue) error {
	var (
		opened bool = true
		m      *protocol.Message
	)

	sCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	defer func() { s.cancel = nil }()

	for opened {
		select {
		case m, opened = <-s.route.MessagesChannel():
			if !opened {
				break
			}
			q.Push(NewRequest(s, m))
		case <-sCtx.Done():
			return nil
		}
	}

	//TODO Cosmin Bogdan returning this error can mean 2 things: overflow of route's channel, or intentional stopping of router / gubled.
	return ErrRouteChannelClosed
}

func (s *subscriber) SetLastID(ID uint64) error {
	//TODO Cosmin Bogdan
	return nil
}

func (s *subscriber) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *subscriber) Encode() ([]byte, error) {
	return json.Marshal(s.data)
}

func GenerateKey(topic string, params map[string]string) string {
	// compute the key from params
	h := sha1.New()
	io.WriteString(h, topic)
	for k, v := range params {
		io.WriteString(h, fmt.Sprintf("%s:%s", k, v))
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:])
}
