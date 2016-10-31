package connector

import (
	"sync"

	"github.com/smancke/guble/server/kvstore"
)

type Manager interface {
	List() []Subscriber
	Add(Subscriber) error
	Update(Subscriber) error
	Remove(Subscriber) error
	Exists(Subscriber) bool
}

type manager struct {
	sync.RWMutex
	schema      string
	kvstore     kvstore.KVStore
	subscribers map[string]Subscriber
}

func NewManager(schema string, kvstore kvstore.KVStore) (Manager, error) {
	sm := &manager{
		schema:      schema,
		kvstore:     kvstore,
		subscribers: make(map[string]Subscriber, 0),
	}

	if err := sm.load(); err != nil {
		return nil, err
	}

	return sm, nil
}

func (sm *manager) load() error {
	// try to load s from kvstore
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

func (sm *manager) List() []Subscriber {
	sm.Lock()
	defer sm.Unlock()

	l := make([]Subscriber, 0, len(sm.subscribers))
	for _, s := range sm.subscribers {
		l = append(l, s)
	}
	return l
}

func (sm *manager) Add(s Subscriber) error {
	sm.Lock()
	defer sm.Unlock()

	if _, found := sm.subscribers[s.Key()]; found {
		return ErrSubscriberExists
	}
	sm.subscribers[s.Key()] = s
	return nil
}

func (sm *manager) Update(s Subscriber) error {
	sm.Lock()
	defer sm.Unlock()
	if _, found := sm.subscribers[s.Key()]; !found {
		return ErrSubscriberDoesntExist
	}

	sm.subscribers[s.Key()] = s
	return nil
}

func (sm *manager) Exists(s Subscriber) bool {
	sm.RLock()
	defer sm.RUnlock()

	_, found := sm.subscribers[s.Key()]
	return found
}

func (sm *manager) Remove(s Subscriber) error {
	sm.Lock()
	defer sm.Unlock()

	if _, found := sm.subscribers[s.Key()]; !found {
		return ErrSubscriberDoesntExist
	}
	delete(sm.subscribers, s.Key())
	return nil
}
