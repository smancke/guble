package store

type MemoryKVStore struct {
	data map[string]map[string][]byte
}

func NewMemoryKVStore() *MemoryKVStore {
	return &MemoryKVStore{
		data: make(map[string]map[string][]byte),
	}
}

func (kvStore *MemoryKVStore) Put(schema, key string, value []byte) error {
	s := kvStore.getSchema(schema)
	s[key] = value
	return nil
}

func (kvStore *MemoryKVStore) Get(schema, key string) (value []byte, exist bool, err error) {
	s := kvStore.getSchema(schema)
	if v, ok := s[key]; ok {
		return v, true, nil
	}
	return nil, false, nil
}

func (kvStore *MemoryKVStore) Delete(schema, key string) error {
	s := kvStore.getSchema(schema)
	delete(s, key)
	return nil
}

func (kvStore *MemoryKVStore) getSchema(schema string) map[string][]byte {
	if s, ok := kvStore.data[schema]; ok {
		return s
	}
	s := make(map[string][]byte)
	kvStore.data[schema] = s
	return s
}
