package store

import (
	"github.com/smancke/guble/guble"
)

// Interface for a persistance backend storing topics
type MessageStore interface {

	// store a message within a partition
	Store(partition string, msg *guble.Message) error

	// fetch a set of messages
	Fetch(FetchRequest)
}

// A fetch request for fetching messages in a MessageStore
type FetchRequest struct {

	// The partition to search for messages
	Partition string

	// The message sequence id to start
	StartId uint64

	// A topic path to filter
	TopicPath guble.Path

	// The maximum number of messages to return
	// AdditionalMessageCount == 0: Only the Message with StartId
	// AdditionalMessageCount >0: Fetch also the next AdditionalMessageCount Messages with a higher MessageId
	// AdditionalMessageCount <0: Fetch also the next AdditionalMessageCount Messages with a lower MessageId
	AdditionalMessageCount int

	// The cannel to send the message back to the receiver
	MessageC chan *guble.Message

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
