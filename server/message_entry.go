package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"
	"strings"
	"time"
)

// The message entry is responsible for handling of all incomming messages
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
	partition := entry.getPartitionFromTopic(msg.Path)

	txCallback := func(msgId uint64) []byte {
		msg.Id = msgId
		msg.PublishingTime = time.Now().Format(time.RFC3339)
		return msg.Bytes()
	}

	if err := entry.messageStore.StoreTx(partition, txCallback); err != nil {
		guble.Err("error storing message in partition %v: %v", partition, err)
		return err
	}

	return entry.router.HandleMessage(msg)
}

func (entry *MessageEntry) getPartitionFromTopic(topicPath guble.Path) string {
	if len(topicPath) > 0 && topicPath[0] == '/' {
		topicPath = topicPath[1:]
	}
	return strings.SplitN(string(topicPath), "/", 2)[0]
}
