package connector

import (
	"sync"

	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/kvstore"
	"github.com/smancke/guble/server/router"
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
	logger.WithField("key", key).Debug("Create generated key")
	if m.Exists(key) {
		logger.WithField("key", key).Debug("Create key exists already")
		return nil, ErrSubscriberExists
	}

	s := NewSubscriber(topic, params, 0)

	logger.WithField("subscriber", s).Debug("Create  newSubscriber created")
	err := m.Add(s)
	if err != nil {
		logger.WithField("error", err.Error()).Debug("Create Manager Add failed")
		return nil, err
	}
	logger.Debug("Create  finished")
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
	logger.WithField("subscriber", s).WithField("lock", m.RWMutex).Debug("Add subscriber before locking")
	err := m.checkSubscribersExist(s)
	if err != nil {
		return err
	}

	if err := m.updateStore(s); err != nil {
		return err
	}

	m.putSubscriberInMap(s)
	logger.WithField("subscriber", s).Debug("Add subscriber after updating store")
	return nil
}

func (m *manager) Update(s Subscriber) error {
	err := m.checkSubscriberDoesNotExists(s)
	if err != nil {
		return err
	}

	m.putSubscriberInMap(s)
	return m.updateStore(s)
}

func (m *manager) checkSubscriberDoesNotExists(s Subscriber) error {
	m.RLock()
	defer m.RUnlock()

	if _, found := m.subscribers[s.Key()]; !found {
		return ErrSubscriberDoesNotExist
	}

	return nil
}

func (m *manager) checkSubscribersExist(s Subscriber) error {
	m.RLock()
	defer m.RUnlock()

	if _, found := m.subscribers[s.Key()]; found {
		return ErrSubscriberExists
	}

	return nil
}

func (m *manager) putSubscriberInMap(s Subscriber) {
	m.Lock()
	defer m.Unlock()
	m.subscribers[s.Key()] = s
}

func (m *manager) deleteSubscriberFromMap(s Subscriber) {
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
	m.cancelSubscribers(s)

	err := m.checkSubscriberDoesNotExists(s)
	if err != nil {
		return err
	}
	m.deleteSubscriberFromMap(s)

	return m.removeStore(s)
}

func (m *manager) cancelSubscribers(s Subscriber) {
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
	logger.WithField("subscriber", s).Debug("UpdateStore")
	return m.kvstore.Put(m.schema, s.Key(), data)
}

func (m *manager) removeStore(s Subscriber) error {
	//TODO MARIAN also remove this logs.
	logger.WithField("subscriber", s).Debug("RemoveStore")
	return m.kvstore.Delete(m.schema, s.Key())
}
