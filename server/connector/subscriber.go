package connector

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/router"
	"github.com/cosminrentea/gobbler/server/store"
)

var (
	ErrSubscriberExists       = errors.New("Subscriber exists.")
	ErrSubscriberDoesNotExist = errors.New("Subscriber does not exist.")

	ErrRouteChannelClosed = errors.New("Subscriber route channel has been closed.")
)

type Subscriber interface {
	// Reset will recreate the route inside the subscribe with the information stored
	// in the subscriber data
	Reset() error
	Key() string
	Route() *router.Route
	Filter(map[string]string) bool
	Loop(context.Context, Queue) error
	SetLastID(ID uint64)
	Cancel()
	Encode() ([]byte, error)
}

type SubscriberData struct {
	Topic  protocol.Path
	Params router.RouteParams
	LastID uint64
}

func (sd *SubscriberData) newRoute() *router.Route {
	var fr *store.FetchRequest
	if sd.LastID > 0 {
		fr = store.NewFetchRequest(sd.Topic.Partition(), sd.LastID, 0, store.DirectionForward, -1)
	}
	return router.NewRoute(router.RouteConfig{
		Path:         sd.Topic,
		RouteParams:  sd.Params,
		FetchRequest: fr,
	})
}

type subscriber struct {
	data SubscriberData

	key    string
	route  *router.Route
	cancel context.CancelFunc
}

func NewSubscriber(topic protocol.Path, params router.RouteParams, lastID uint64) Subscriber {
	return NewSubscriberFromData(SubscriberData{
		Topic:  topic,
		Params: params,
		LastID: lastID,
	})
}

func NewSubscriberFromData(data SubscriberData) Subscriber {
	return &subscriber{
		data:  data,
		route: data.newRoute(),
	}
}

func NewSubscriberFromJSON(data []byte) (Subscriber, error) {
	sd := SubscriberData{}
	err := json.Unmarshal(data, &sd)
	if err != nil {
		return nil, err
	}
	return NewSubscriberFromData(sd), nil
}

func (s *subscriber) String() string {
	return s.Key()
}

func (s *subscriber) Reset() error {
	s.route = s.data.newRoute()
	s.cancel = nil
	return nil
}

func (s *subscriber) Key() string {
	if s.key == "" {
		s.key = GenerateKey(string(s.data.Topic), s.data.Params)
	}
	return s.key
}

func (s *subscriber) Filter(filters map[string]string) bool {
	return s.route.Filter(filters)
}

func (s *subscriber) Route() *router.Route {
	return s.route
}

func (s *subscriber) Loop(ctx context.Context, q Queue) error {
	var m *protocol.Message
	sCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	defer func() { s.cancel = nil }()

	opened := true
	for opened {
		select {
		case m, opened = <-s.route.MessagesChannel():
			if !opened {
				break
			}

			q.Push(NewRequest(s, m))
		case <-sCtx.Done():
			// If the parent context is still running then only this subscriber context
			// has been cancelled
			if ctx.Err() == nil {
				return sCtx.Err()
			}
			return nil
		}
	}

	//TODO Cosmin Bogdan returning this error can mean 2 things: overflow of route's channel, or intentional stopping of router / gubled.
	return ErrRouteChannelClosed
}

func (s *subscriber) SetLastID(ID uint64) {
	s.data.LastID = ID
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

	// compute the hash with ordered params keys
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		io.WriteString(h, fmt.Sprintf("%s:%s", k, params[k]))
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:])
}
