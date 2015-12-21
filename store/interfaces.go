package store

// Interface for a persistance backend, storing key value pairs.
type KVStore interface {

	// Store an entry in the key value store
	Put(schema, key string, value []byte) error

	// Fetch one entry
	Get(schema, key string) (value []byte, exist bool, err error)

	// Delete an entry
	Delete(schema, key string) error

	// Iterates over all entries in the key value store.
	// The result will be send to the channel, which is closes after the last entry.
	// For simplicity of the return type is an string array with key, value.
	// If you have binary values, you can savely cast back to []byte.
	Iterate(schema string, keyPrefix string) (entries chan [2]string)

	// Iterates over all keys in the key value store.
	// The keys will be send to the channel, which is closes after the last entry.
	IterateKeys(schema string, keyPrefix string) (keys chan string)
}
