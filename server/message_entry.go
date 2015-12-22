package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"
	"strconv"
	"strings"
	"sync"
	"time"
)

const TOPIC_SCHEMA = "topic_sequence"

// The message entry is responsible for handling of all incomming messages
// It takes a raw message, calculates the message id and decides how to handle
// the message within the service.
// Ass all the chainable message handler, it supports the MessageSink interface.
type MessageEntry struct {
	router             MessageSink
	topicSequences     map[string]uint64
	topicSequencesLock sync.RWMutex
	kvStore            store.KVStore
}

func NewMessageEntry(router MessageSink) *MessageEntry {
	return &MessageEntry{
		router:         router,
		topicSequences: make(map[string]uint64),
	}
}

func (entry *MessageEntry) Start() {
	go entry.startSequenceSync()
}

func (entry *MessageEntry) SetKVStore(kvStore store.KVStore) {
	entry.kvStore = kvStore
}

// Take the message and forward it to the router.
func (entry *MessageEntry) HandleMessage(message *guble.Message) {
	message.Id = entry.nextIdForTopic(string(message.Path))
	message.PublishingTime = time.Now().Format(time.RFC3339)
	entry.router.HandleMessage(message)
}

func (entry *MessageEntry) nextIdForTopic(topicPath string) uint64 {
	if len(topicPath) > 0 && topicPath[0] == '/' {
		topicPath = topicPath[1:]
	}
	topicKey := strings.SplitN(topicPath, "/", 2)[0]

	entry.topicSequencesLock.Lock()
	defer entry.topicSequencesLock.Unlock()

	sequenceValue, exist := entry.topicSequences[topicKey]
	if !exist {
		// TODO: What should we do on an error, here? For now: start by 0
		if val, existInKVStore, err := entry.kvStore.Get(TOPIC_SCHEMA, topicKey); existInKVStore && err == nil {
			sequenceValue, err = strconv.ParseUint(string(val), 10, 0)
		}
	}
	entry.topicSequences[topicKey] = sequenceValue + 1
	return sequenceValue + 1
}

func (entry *MessageEntry) startSequenceSync() {
	lastSyncValues := make(map[string]uint64)
	topicsToUpdate := []string{}

	for {
		entry.topicSequencesLock.Lock()
		topicsToUpdate = topicsToUpdate[:0]
		for topic, seq := range entry.topicSequences {
			if lastSyncValues[topic] != seq {
				topicsToUpdate = append(topicsToUpdate, topic)
			}
		}
		entry.topicSequencesLock.Unlock()

		for _, topic := range topicsToUpdate {
			entry.topicSequencesLock.Lock()
			latestValue := entry.topicSequences[topic]
			entry.topicSequencesLock.Unlock()

			lastSyncValues[topic] = latestValue
			entry.kvStore.Put(TOPIC_SCHEMA, topic, []byte(strconv.FormatUint(latestValue, 10)))
		}
		time.Sleep(time.Millisecond * 100)
	}

}
