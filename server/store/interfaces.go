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
	StoreMessage(*protocol.Message, int) (int, error)

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
	GenerateNextMsgID(partition string, nodeID int) (uint64, int64, error)
}
