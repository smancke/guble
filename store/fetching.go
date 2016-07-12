package store

// FetchRequest is used for fetching messages in a MessageStore.
type FetchRequest struct {

	// Partition is the Store name to search for messages
	Partition string

	// StartId is the message sequence id to start
	StartID uint64

	//EndId is the message sequence id to finish. If0  will not be used.
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

	// ErrorCallback is a Callback if an error occurs
	ErrorC chan error

	// Through the start callback, the total number or result
	// is returned, before sending the first message.
	// The Fetch() methods blocks on putting the number to the start callback.
	StartC chan int
}

type FetchedMessage struct {
	ID      uint64
	Message []byte
}
