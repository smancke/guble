package store

import "github.com/smancke/guble/protocol"

// MessageStore is an interface for a persistence backend storing topics.
type MessageStore interface {

	// Store a message within a partition.
	// The message id must be equal to MaxMessageId +1.
	// So the caller has to maintain the consistence between
	// fetching an id and storing the message.
	Store(partition string, messageID uint64, data []byte) error

	// Generates a new ID for the message if it's new and stores it
	// Returns the size of the new message or error
	// Takes the message and cluster node ID as parameters.
	StoreMessage(*protocol.Message, uint8) (int, error)

	// Fetch fetches a set of messages.
	// The results, as well as errors are communicated asynchronously using
	// the channels, supplied by the FetchRequest.
	Fetch(FetchRequest)

	// MaxMessageId returns the highest message id for a particular partition
	MaxMessageID(partition string) (uint64, error)

	// DoInTx executes the supplied function within the locking context of the message partition.
	// This ensures, that wile the code is executed, no change to the supplied maxMessageId can occur.
	// The error result if the fnToExecute or an error while locking will be returned by DoInTx.
	DoInTx(partition string, fnToExecute func(uint64) error) error

	// GenerateNextMsgId generates a new message ID based on a timestamp in a strictly monotonically order
	GenerateNextMsgID(partition string, nodeID uint8) (uint64, int64, error)

	// Partitions() ([]*MessagePartition, error)

}

type MessageAndID struct {
	ID      uint64
	Message []byte
}

// FetchRequest is used for fetching messages in a MessageStore.
type FetchRequest struct {

	// Partition is the Store name to search for messages
	Partition string

	// StartId is the message sequence id to start
	StartID uint64

	//EndId is the message sequence id to finish. If  will not be used.
	EndID uint64

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
	MessageC chan FetchedMessage

	// ErrorC is a channel if an error occurs
	ErrorC chan error

	// StartC Through this channel , the total number or result
	// is returned, before sending the first message.
	// The Fetch() methods blocks on putting the number to the start channel.
	StartC chan int
}

// FetchedMessage is a struct containing a pair: guble Message and its ID.
type FetchedMessage struct {
	ID      uint64
	Message []byte
}
