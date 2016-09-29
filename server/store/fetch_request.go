package store

const (
	DirectionOneMessage FetchDirection = 0
	DirectionForward    FetchDirection = 1
	DirectionBackwards  FetchDirection = -1
)

type FetchDirection int

// FetchedMessage is a struct containing a pair: guble Message and its ID.
type FetchedMessage struct {
	ID      uint64
	Message []byte
}

// FetchRequest is used for fetching messages in a MessageStore.
type FetchRequest struct {

	// Partition is the Store name to search for messages
	Partition string

	// StartID is the message sequence id to start
	StartID uint64

	// EndID is the message sequence id to finish. If  will not be used.
	EndID uint64

	// Direction has 3 possible values:
	// Direction == 0: Only the Message with StartId
	// Direction == 1: Fetch also the next Count Messages with a higher MessageId
	// Direction == -1: Fetch also the next Count Messages with a lower MessageId
	Direction int

	// Count is the maximum number of messages to return
	Count int

	// MessageC is the channel to send the message back to the receiver
	MessageC chan *FetchedMessage

	// ErrorC is a channel if an error occurs
	ErrorC chan error

	// StartC Through this channel , the total number or result
	// is returned, before sending the first message.
	// The Fetch() methods blocks on putting the number to the start channel.
	StartC chan int
}

func NewFetchRequest(partition string, start, end uint64, direction FetchDirection) *FetchRequest {
	return &FetchRequest{
		Partition: partition,
		StartID:   start,
		EndID:     end,
		Direction: int(direction),

		StartC:   make(chan int),
		MessageC: make(chan *FetchedMessage, 10),
		ErrorC:   make(chan error),
	}
}

func (fr *FetchRequest) Messages() <-chan *FetchedMessage {
	return fr.MessageC
}

func (fr *FetchRequest) Errors() <-chan error {
	return fr.ErrorC
}

func (fr *FetchRequest) Push(id uint64, message []byte) {
	fr.PushFetchMessage(&FetchedMessage{id, message})
}

func (fr *FetchRequest) PushFetchMessage(fm *FetchedMessage) {

}

func (fr *FetchRequest) PushError(err error) {
	fr.ErrorC <- err
}
