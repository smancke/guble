package server

import (
	guble "github.com/smancke/guble/guble"
	"strings"
	"sync"
)

// The message entry is responsible for handling of all incomming messages
// It takes a raw message, calculates the message id and decides how to handle
// the message within the service.
// Ass all the chainable message handler, it supports the MessageSink interface.
type MessageEntry struct {
	router             MessageSink
	topicSequences     map[string]uint64
	topicSequencesLock sync.RWMutex
}

func NewMessageEntry(router MessageSink) *MessageEntry {
	return &MessageEntry{
		router:         router,
		topicSequences: make(map[string]uint64),
	}
}

// Take the message and forward it to the router.
func (entry *MessageEntry) HandleMessage(message *guble.Message) {
	message.Id = entry.nextIdForTopic(string(message.Path))
	entry.router.HandleMessage(message)
}

func (entry *MessageEntry) nextIdForTopic(topicPath string) uint64 {
	if len(topicPath) > 0 && topicPath[0] == '/' {
		topicPath = topicPath[1:]
	}
	topicKey := strings.SplitN(topicPath, "/", 2)[0]

	entry.topicSequencesLock.Lock()
	defer entry.topicSequencesLock.Unlock()

	sequenceValue := entry.topicSequences[topicKey]
	entry.topicSequences[topicKey] = sequenceValue + 1
	return sequenceValue + 1
}
