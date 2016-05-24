package store

import (
	"strings"
	"sync"
)

type MemoryKVStore struct {
	data  map[string]map[string][]byte
	mutex *sync.RWMutex
}

func NewMemoryKVStore() *MemoryKVStore {
	return &MemoryKVStore{
		data:  make(map[string]map[string][]byte),
		mutex: &sync.RWMutex{},
	}
}

func (kvStore *MemoryKVStore) Put(schema, key string, value []byte) error {
	kvStore.mutex.Lock()
	defer kvStore.mutex.Unlock()
	s := kvStore.getSchema(schema)
	s[key] = value
	return nil
}

func (kvStore *MemoryKVStore) Get(schema, key string) (value []byte, exist bool, err error) {
	kvStore.mutex.Lock()
	defer kvStore.mutex.Unlock()
	s := kvStore.getSchema(schema)
	if v, ok := s[key]; ok {
		return v, true, nil
	}
	return nil, false, nil
}

func (kvStore *MemoryKVStore) Delete(schema, key string) error {
	kvStore.mutex.Lock()
	defer kvStore.mutex.Unlock()
	s := kvStore.getSchema(schema)
	delete(s, key)
	return nil
}

// TODO: this can lead to a deadlock,
// if the consumer modifies the store while receiving and the channel blocks
func (kvStore *MemoryKVStore) Iterate(schema string, keyPrefix string) (entries chan [2]string) {
	responseChan := make(chan [2]string, 100)
	kvStore.mutex.Lock()
	s := kvStore.getSchema(schema)
	kvStore.mutex.Unlock()
	go func() {
		kvStore.mutex.Lock()
		for key, value := range s {
			if strings.HasPrefix(key, keyPrefix) {
				responseChan <- [2]string{key, string(value)}
			}
		}
		kvStore.mutex.Unlock()
		close(responseChan)
	}()
	return responseChan
}

// TODO: this can lead to a deadlock,
// if the consumer modifies the store while receiving and the channel blocks
func (kvStore *MemoryKVStore) IterateKeys(schema string, keyPrefix string) chan string {
	responseChan := make(chan string, 100)
	kvStore.mutex.Lock()
	s := kvStore.getSchema(schema)
	kvStore.mutex.Unlock()
	go func() {
		kvStore.mutex.Lock()
		for key, _ := range s {
			if strings.HasPrefix(key, keyPrefix) {
				responseChan <- key
			}
		}
		kvStore.mutex.Unlock()

		close(responseChan)
	}()
	return responseChan
}

func (kvStore *MemoryKVStore) getSchema(schema string) map[string][]byte {
	if s, ok := kvStore.data[schema]; ok {
		return s
	}
	s := make(map[string][]byte)
	kvStore.data[schema] = s
	return s
}
