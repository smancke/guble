package store

import (
	"errors"
	"math"
	"sync"
)

var ErrRequestDone = errors.New("Fetch request is done")

const (
	DirectionOneMessage FetchDirection = 0
	DirectionForward    FetchDirection = 1
	DirectionBackwards  FetchDirection = -1

	// TODO Bogdan decide the channel size and if should be customizable
	FetchBufferSize = 10
)

type FetchDirection int

// FetchedMessage is a struct containing a pair: guble Message and its ID.
type FetchedMessage struct {
	ID      uint64
	Message []byte
}

// FetchRequest is used for fetching messages in a MessageStore.
type FetchRequest struct {
	sync.RWMutex

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
	Direction FetchDirection

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

	done bool
}

// Creates a new FetchRequest pointer initialized with provided values
// if `count` is negative will be set to MaxInt32
func NewFetchRequest(partition string, start, end uint64, direction FetchDirection, count int) *FetchRequest {
	if count < 0 {
		count = math.MaxInt32
	}
	return &FetchRequest{
		Partition: partition,
		StartID:   start,
		EndID:     end,
		Direction: direction,

		Count: count,
	}
}

func (fr *FetchRequest) Init() {
	fr.Lock()
	defer fr.Unlock()
	fr.done = false

	fr.StartC = make(chan int)
	fr.MessageC = make(chan *FetchedMessage, FetchBufferSize)
	fr.ErrorC = make(chan error)
}

// Ready returns the count of messages that will be returned meaning that
// the fetch is starting. It reads the number from the StartC channel.
func (fr *FetchRequest) Ready() int {
	return <-fr.StartC
}

func (fr *FetchRequest) Messages() <-chan *FetchedMessage {
	return fr.MessageC
}

func (fr *FetchRequest) Errors() <-chan error {
	return fr.ErrorC
}

func (fr *FetchRequest) Error(err error) {
	fr.ErrorC <- err
}

func (fr *FetchRequest) Push(id uint64, message []byte) {
	fr.PushFetchMessage(&FetchedMessage{id, message})
}

func (fr *FetchRequest) PushFetchMessage(fm *FetchedMessage) {
	fr.MessageC <- fm
}

func (fr *FetchRequest) PushError(err error) {
	fr.ErrorC <- err
}

func (fr *FetchRequest) IsDone() bool {
	fr.RLock()
	defer fr.RUnlock()
	return fr.done
}

func (fr *FetchRequest) Done() {
	fr.Lock()
	defer fr.Unlock()
	fr.done = true

	close(fr.MessageC)
}
