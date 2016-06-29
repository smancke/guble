package store

// MessageStore is an interface for a persistence backend storing topics.
type MessageStore interface {

	// Store a message within a partition.
	// The message id must be equal to MaxMessageId +1.
	// So the caller has to maintain the consistence between
	// fetching an id and storing the message.
	Store(partition string, msgID uint64, msg []byte) error

	// StoreTx retrieves the next available id and stores the message in one atomic transaction
	StoreTx(partition string,
		callback func(msgID uint64) (msg []byte)) error

	// Fetch fetches a set of messages.
	// The results, as well as errors are communicated asynchronously using
	// the channels, supplied by the FetchRequest.
	Fetch(FetchRequest)

	// MaxMessageId returns the highest message id for a particular partition
	MaxMessageID(partition string) (uint64, error)

	// DoInTx executes the supplied function within the locking context of the message partition.
	// This ensures, that wile the code is executed, no change to the supplied maxMessageId can occur.
	// The error result if the fnToExecute or an error while locking will be returned by DoInTx.
	DoInTx(partition string, fnToExecute func(maxMessageId uint64) error) error

	//Check if the current messageStore is having enough space to save on Disk
	Check() error
}

type MessageAndID struct {
	ID      uint64
	Message []byte
}

// FetchRequest is used for fetching messages in a MessageStore.
type FetchRequest struct {

	// Partition is the Store name to search for messages
	Partition string

	// StartID is the message sequence id to start
	StartID uint64

	// Direction has 3 possible values:
	// Direction == 0: Only the Message with StartId
	// Direction == 1: Fetch also the next Count Messages with a higher MessageId
	// Direction == -1: Fetch also the next Count Messages with a lower MessageId
	Direction int

	// Count is the maximum number of messages to return
	Count int

	// Prefix is the message prefix to filter
	Prefix []byte

	// MessageC is the channel to send the message back to the receiver
	MessageC chan MessageAndID

	// ErrorCallback is a Callback if an error occurs
	ErrorC chan error

	// Through the start callback, the total number or result
	// is returned, before sending the first message.
	// The Fetch() methods blocks on putting the number to the start callback.
	StartC chan int
}

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
