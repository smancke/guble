package connector

import (
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/kvstore"
	"github.com/smancke/guble/server/router"
	"sync"
)

type Manager interface {
	Load() error
	List() []Subscriber
	Find(string) Subscriber
	Exists(string) bool
	Create(protocol.Path, router.RouteParams) (Subscriber, error)
	Add(Subscriber) error
	Update(Subscriber) error
	Remove(Subscriber) error
}

type manager struct {
	sync.RWMutex
	schema      string
	kvstore     kvstore.KVStore
	subscribers map[string]Subscriber
}

func NewManager(schema string, kvstore kvstore.KVStore) Manager {
	return &manager{
		schema:      schema,
		kvstore:     kvstore,
		subscribers: make(map[string]Subscriber, 0),
	}
}

func (m *manager) Load() error {
	// try to load s from kvstore
	entries := m.kvstore.Iterate(m.schema, "")
	for e := range entries {
		subscriber, err := NewSubscriberFromJSON([]byte(e[1]))
		if err != nil {
			return err
		}
		m.subscribers[subscriber.Key()] = subscriber
	}
	return nil
}

func (m *manager) Find(key string) Subscriber {
	if s, exists := m.subscribers[key]; exists {
		return s
	}
	return nil
}

func (m *manager) Create(topic protocol.Path, params router.RouteParams) (Subscriber, error) {
	key := GenerateKey(string(topic), params)
	if m.Exists(key) {
		return nil, ErrSubscriberExists
	}

	s := NewSubscriber(topic, params, nil)
	err := m.Add(s)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (m *manager) List() []Subscriber {
	m.Lock()
	defer m.Unlock()

	l := make([]Subscriber, 0, len(m.subscribers))
	for _, s := range m.subscribers {
		l = append(l, s)
	}
	return l
}

func (m *manager) Add(s Subscriber) error {
	m.Lock()
	defer m.Unlock()

	if _, found := m.subscribers[s.Key()]; found {
		return ErrSubscriberExists
	}
	m.subscribers[s.Key()] = s

	err := m.updateStore(s)
	if err != nil {
		return err
	}

	return nil
}

func (m *manager) Update(s Subscriber) error {
	m.Lock()
	defer m.Unlock()
	if _, found := m.subscribers[s.Key()]; !found {
		return ErrSubscriberDoesNotExist
	}

	m.subscribers[s.Key()] = s
	err := m.updateStore(s)
	if err != nil {
		return err
	}

	return nil
}

func (m *manager) Exists(key string) bool {
	m.RLock()
	defer m.RUnlock()

	_, found := m.subscribers[key]
	return found
}

func (m *manager) Remove(s Subscriber) error {
	m.Lock()
	defer m.Unlock()

	if _, found := m.subscribers[s.Key()]; !found {
		return ErrSubscriberDoesNotExist
	}
	delete(m.subscribers, s.Key())
	m.removeStore(s)
	return nil
}

func (m *manager) updateStore(s Subscriber) error {
	data, err := s.Encode()
	if err != nil {
		return err
	}

	err = m.kvstore.Put(m.schema, s.Key(), data)
	if err != nil {
		return err
	}
	return nil
}

func (m *manager) removeStore(s Subscriber) error {
	return m.kvstore.Delete(m.schema, s.Key())
}
