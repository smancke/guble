package kvstore

// KVStore is an interface for a persistence backend, storing key-value pairs.
type KVStore interface {

	// Put stores an entry in the key-value store
	Put(schema, key string, value []byte) error

	// Get fetches one entry
	Get(schema, key string) (value []byte, exist bool, err error)

	// Delete an entry
	Delete(schema, key string) error

	// Iterate iterates over all entries in the key value store.
	// The result will be sent to the channel, which is closed after the last entry.
	// For simplicity, the return type is an string array with key, value.
	// If you have binary values, you can safely cast back to []byte.
	Iterate(schema, keyPrefix string) (entries chan [2]string)

	// IterateKeys iterates over all keys in the key value store.
	// The keys will be sent to the channel, which is closed after the last entry.
	IterateKeys(schema, keyPrefix string) (keys chan string)
}
