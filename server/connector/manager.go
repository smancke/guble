package connector

import (
	"sync"

	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/kvstore"
	"github.com/cosminrentea/gobbler/server/router"
)

type Manager interface {
	Load() error
	List() []Subscriber
	Filter(map[string]string) []Subscriber
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
	m.RLock()
	defer m.RUnlock()

	if s, exists := m.subscribers[key]; exists {
		return s
	}
	return nil
}

func (m *manager) Create(topic protocol.Path, params router.RouteParams) (Subscriber, error) {
	key := GenerateKey(string(topic), params)
	//TODO MARIAN  remove this logs   when 503 is done.
	logger.WithField("key", key).Info("Create generated key")
	if m.Exists(key) {
		logger.WithField("key", key).Info("Create key exists already")
		return nil, ErrSubscriberExists
	}

	s := NewSubscriber(topic, params, 0)

	logger.WithField("subscriber", s).Info("Created new subscriber")
	err := m.Add(s)
	if err != nil {
		logger.WithField("error", err.Error()).Info("Create Manager Add failed")
		return nil, err
	}
	logger.Info("Create finished")
	return s, nil
}

func (m *manager) List() []Subscriber {
	m.RLock()
	defer m.RUnlock()

	l := make([]Subscriber, 0, len(m.subscribers))
	for _, s := range m.subscribers {
		l = append(l, s)
	}
	return l
}

func (m *manager) Filter(filters map[string]string) (subscribers []Subscriber) {
	m.RLock()
	defer m.RUnlock()

	for _, s := range m.subscribers {
		if s.Filter(filters) {
			subscribers = append(subscribers, s)
		}
	}
	return
}

func (m *manager) Add(s Subscriber) error {
	logger.WithField("subscriber", s).WithField("lock", m.RWMutex).Info("Add subscriber started")

	if m.Exists(s.Key()) {
		return ErrSubscriberExists
	}

	if err := m.updateStore(s); err != nil {
		return err
	}

	m.putSubscriber(s)
	logger.WithField("subscriber", s).Info("Add subscriber finished")
	return nil
}

func (m *manager) Update(s Subscriber) error {
	logger.WithField("subscriber", s).Info("Update subscriber started")
	if !m.Exists(s.Key()) {
		return ErrSubscriberDoesNotExist
	}

	err := m.updateStore(s)
	if err != nil {
		return err
	}

	m.putSubscriber(s)
	logger.WithField("subscriber", s).Info("Update subscriber finished")
	return nil
}

func (m *manager) putSubscriber(s Subscriber) {
	m.Lock()
	defer m.Unlock()
	m.subscribers[s.Key()] = s
}

func (m *manager) deleteSubscriber(s Subscriber) {
	m.Lock()
	defer m.Unlock()
	delete(m.subscribers, s.Key())
}

func (m *manager) Exists(key string) bool {
	m.RLock()
	defer m.RUnlock()

	_, found := m.subscribers[key]
	return found
}

func (m *manager) Remove(s Subscriber) error {
	logger.WithField("subscriber", s).Info("Remove subscriber started")
	m.cancelSubscriber(s)

	if !m.Exists(s.Key()) {
		return ErrSubscriberDoesNotExist
	}

	err := m.removeStore(s)
	if err != nil {
		return err
	}

	m.deleteSubscriber(s)
	logger.WithField("subscriber", s).Info("Remove subscriber finished")
	return nil
}

func (m *manager) cancelSubscriber(s Subscriber) {
	m.Lock()
	defer m.Unlock()

	s.Cancel()
}

func (m *manager) updateStore(s Subscriber) error {
	data, err := s.Encode()
	if err != nil {
		return err
	}
	//TODO MARIAN also remove this logs.
	logger.WithField("subscriber", s).Info("UpdateStore")
	return m.kvstore.Put(m.schema, s.Key(), data)
}

func (m *manager) removeStore(s Subscriber) error {
	//TODO MARIAN also remove this logs.
	logger.WithField("subscriber", s).Info("RemoveStore")
	return m.kvstore.Delete(m.schema, s.Key())
}
