package store

// Interface for a persistance backend storing topics
type MessageStore interface {

	// Store a message within a partition
	Store(partition string, msgId uint64, msg []byte) error

	// fetch a set of messages
	Fetch(FetchRequest)
}

// A fetch request for fetching messages in a MessageStore
type FetchRequest struct {

	// The Store name to search for messages
	Partition string

	// The message sequence id to start
	StartId uint64

	// Direction == 0: Only the Message with StartId
	// Direction == 1: Fetch also the next Count Messages with a higher MessageId
	// Direction == -1: Fetch also the next Count Messages with a lower MessageId
	Direction int

	// The maximum number of messages to return
	Count int

	// The message prefix to filter
	Prefix []byte

	// The cannel to send the message back to the receiver
	MessageC chan []byte

	// A Callback if an error occures
	ErrorCallback chan error
}

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
