package store

// Interface for a persistance backend storing key value pairs
type KVStore interface {
	Put(schema, key string, value []byte) error
	Get(schema, key string) (value []byte, exist bool, err error)
	Delete(schema, key string) error
	IterateKeys(schema string, keyPrefix string) (keys chan string)
}
