package connector

import (
	"sync"

	"github.com/smancke/guble/server/kvstore"
)

type SubscriptionManager interface {
	List() []Subscriber
	Add(Subscriber) error
	Update(Subscriber) error
	Remove(Subscriber) error
	Exists(Subscriber) bool
}

type subscriptionManager struct {
	sync.RWMutex
	schema      string
	kvstore     kvstore.KVStore
	subscribers map[string]Subscriber
}

func NewSubscriptionManager(schema string, kvstore kvstore.KVStore) (SubscriptionManager, error) {
	sm := &subscriptionManager{
		schema:      schema,
		kvstore:     kvstore,
		subscribers: make(map[string]Subscriber, 0),
	}

	if err := sm.load(); err != nil {
		return nil, err
	}

	return sm, nil
}

func (sm *subscriptionManager) load() error {
	// try to load subscriptions from kvstore
	entries := sm.kvstore.Iterate(sm.schema, "")
	for e := range entries {
		subscriber, err := NewSubscriberFromJSON([]byte(e[1]))
		if err != nil {
			return err
		}
		sm.subscribers[subscriber.Key()] = subscriber
	}
	return nil
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
		return ErrSubscriberDoesNotExist
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
		return ErrSubscriberDoesNotExist
	}
	delete(sm.subscribers, s.Key())
	return nil
}
