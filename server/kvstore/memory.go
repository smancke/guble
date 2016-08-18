package kvstore

import (
	"strings"
	"sync"
)

// MemoryKVStore is a struct representing an in-memory key-value store.
type MemoryKVStore struct {
	data  map[string]map[string][]byte
	mutex *sync.RWMutex
}

// NewMemoryKVStore returns a new configured MemoryKVStore.
func NewMemoryKVStore() *MemoryKVStore {
	return &MemoryKVStore{
		data:  make(map[string]map[string][]byte),
		mutex: &sync.RWMutex{},
	}
}

// Put implements the `kvstore` Put func.
func (kvStore *MemoryKVStore) Put(schema, key string, value []byte) error {
	kvStore.mutex.Lock()
	defer kvStore.mutex.Unlock()
	s := kvStore.getSchema(schema)
	s[key] = value
	return nil
}

// Get implements the `kvstore` Get func.
func (kvStore *MemoryKVStore) Get(schema, key string) ([]byte, bool, error) {
	kvStore.mutex.Lock()
	defer kvStore.mutex.Unlock()
	s := kvStore.getSchema(schema)
	if v, ok := s[key]; ok {
		return v, true, nil
	}
	return nil, false, nil
}

// Delete implements the `kvstore` Delete func.
func (kvStore *MemoryKVStore) Delete(schema, key string) error {
	kvStore.mutex.Lock()
	defer kvStore.mutex.Unlock()
	s := kvStore.getSchema(schema)
	delete(s, key)
	return nil
}

// Iterate iterates over the key-value pairs in the schema, with keys matching the keyPrefix.
// TODO: this can lead to a deadlock, if the consumer modifies the store while receiving and the channel blocks
func (kvStore *MemoryKVStore) Iterate(schema string, keyPrefix string) chan [2]string {
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

// IterateKeys iterates over the keys in the schema, matching the keyPrefix.
// TODO: this can lead to a deadlock, if the consumer modifies the store while receiving and the channel blocks
func (kvStore *MemoryKVStore) IterateKeys(schema string, keyPrefix string) chan string {
	responseChan := make(chan string, 100)
	kvStore.mutex.Lock()
	s := kvStore.getSchema(schema)
	kvStore.mutex.Unlock()
	go func() {
		kvStore.mutex.Lock()
		for key := range s {
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
