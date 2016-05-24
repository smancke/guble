package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"

	"time"
)

// The message entry is responsible for handling of all incoming messages
// It takes a raw message, calculates the message id and decides how to handle
// the message within the service.
// Ass all the chainable message handler, it supports the MessageSink interface.
type MessageEntry struct {
	router       MessageSink
	messageStore store.MessageStore
}

func NewMessageEntry(router MessageSink) *MessageEntry {
	return &MessageEntry{
		router: router,
	}
}

func (entry *MessageEntry) SetMessageStore(messageStore store.MessageStore) {
	entry.messageStore = messageStore
}

// Take the message and forward it to the router.
func (entry *MessageEntry) HandleMessage(msg *guble.Message) error {
	txCallback := func(msgId uint64) []byte {
		msg.Id = msgId
		msg.PublishingTime = time.Now().Unix()
		return msg.Bytes()
	}

	if err := entry.messageStore.StoreTx(msg.Path.Partition(), txCallback); err != nil {
		guble.Err("error storing message in partition %v: %v", msg.Path.Partition(), err)
		return err
	}

	return entry.router.HandleMessage(msg)
}
