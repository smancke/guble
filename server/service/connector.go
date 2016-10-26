package service

import (
	"context"
	"errors"
	"sync"

	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/router"
)

var (
	ErrSubscriberExists       = errors.New("Subscriber exists.")
	ErrSubscriberDoesntExists = errors.New("Subscribers doesn't exist.")
)

type Subscriber interface {
	Key() string
	Route() *router.Route
	Loop(context.Context, chan *protocol.Message) error
}

type SubscriptionManager interface {
	List() []Subscriber
	Add(Subscriber) error
	Update(Subscriber) error
	Remove(Subscriber) error
	Exists(Subscriber) bool
}

type subscriptionManager struct {
	sync.RWMutex
	subscribers map[string]Subscriber
}

func NewSubscriptionManager() SubscriptionManager {
	return &subscriptionManager{
		subscribers: make(map[string]Subscriber, 0),
	}
}

func (sm *subscriptionManager) List() []Subscriber {
	sm.Lock()
	defer sm.Unlock()

	l := make([]Subscriber, 0, len(sm.subscribers))
	for _, s := range sm.subscribers {
		l = append(l, s)
	}
	return l
}

func (sm *subscriptionManager) Add(s Subscriber) error {
	sm.Lock()
	defer sm.Unlock()

	if _, found := sm.subscribers[s.Key()]; found {
		return ErrSubscriberExists
	}
	sm.subscribers[s.Key()] = s
	return nil
}

func (sm *subscriptionManager) Update(s Subscriber) error {
	sm.Lock()
	defer sm.Unlock()
	if _, found := sm.subscribers[s.Key()]; !found {
		return ErrSubscriberDoesntExists
	}

	sm.subscribers[s.Key()] = s
	return nil
}

func (sm *subscriptionManager) Exists(s Subscriber) bool {
	sm.RLock()
	defer sm.RUnlock()

	_, found := sm.subscribers[s.Key()]
	return found
}

func (sm *subscriptionManager) Remove(s Subscriber) error {
	sm.Lock()
	defer sm.Unlock()

	if _, found := sm.subscribers[s.Key()]; !found {
		return ErrSubscriberDoesntExists
	}
	delete(sm.subscribers, s.Key())
	return nil
}
