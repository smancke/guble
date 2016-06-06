package store

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

const TOPIC_SCHEMA = "topic_sequence"

// DummyMessageStore is a minimal implementation of the MessageStore interface.
// Everything it does is storing the message ids in the key value store to
// ensure a monotonic incremented id.
// It is intended for testing and demo purpose, as well as dummy for services without persistence.
// TODO: implement a simple logic to preserve the last N messages
type DummyMessageStore struct {
	topicSequences     map[string]uint64
	topicSequencesLock sync.RWMutex
	kvStore            KVStore
	isSyncStarted      bool
	// used to send the stop request to the syc goroutine
	stopC chan bool
	// answer from the syc goroutine, when it is stopped
	stoppedC       chan bool
	idSyncDuration time.Duration
}

func NewDummyMessageStore(kvStore KVStore) *DummyMessageStore {
	return &DummyMessageStore{
		topicSequences: make(map[string]uint64),
		kvStore:        kvStore,
		idSyncDuration: time.Millisecond * 100,
		stopC:          make(chan bool, 1),
		stoppedC:       make(chan bool, 1),
	}
}

func (fms *DummyMessageStore) Start() error {
	go fms.startSequenceSync()
	fms.isSyncStarted = true

	return nil
}

func (fms *DummyMessageStore) Stop() error {
	if !fms.isSyncStarted {
		return nil
	}
	fms.stopC <- true
	<-fms.stoppedC
	return nil
}

func (fms *DummyMessageStore) StoreTx(partition string,
	callback func(msgId uint64) (msg []byte)) error {

	fms.topicSequencesLock.Lock()
	defer fms.topicSequencesLock.Unlock()

	msgId, err := fms.maxMessageId(partition)
	if err != nil {
		return err
	}
	msgId++
	return fms.store(partition, msgId, callback(msgId))
}

func (fms *DummyMessageStore) Store(partition string, msgId uint64, msg []byte) error {
	fms.topicSequencesLock.Lock()
	defer fms.topicSequencesLock.Unlock()
	return fms.store(partition, msgId, msg)
}

func (fms *DummyMessageStore) store(partition string, msgId uint64, msg []byte) error {
	maxId, err := fms.maxMessageId(partition)
	if err != nil {
		return err
	}
	if msgId > 1+maxId {
		return fmt.Errorf("Invalid message id for partition %v. Next id should be %v, but was %q", partition, 1+maxId, msgId)
	}
	fms.setId(partition, msgId)
	return nil
}

// Fetch does nothing in this dummy implementation
func (fms *DummyMessageStore) Fetch(req FetchRequest) {
}

func (fms *DummyMessageStore) MaxMessageId(partition string) (uint64, error) {
	fms.topicSequencesLock.Lock()
	defer fms.topicSequencesLock.Unlock()
	return fms.maxMessageId(partition)
}

func (fms *DummyMessageStore) DoInTx(partition string, fnToExecute func(maxMessageId uint64) error) error {
	fms.topicSequencesLock.Lock()
	defer fms.topicSequencesLock.Unlock()
	maxId, err := fms.maxMessageId(partition)
	if err != nil {
		return err
	}
	return fnToExecute(maxId)
}

func (fms *DummyMessageStore) maxMessageId(partition string) (uint64, error) {

	sequenceValue, exist := fms.topicSequences[partition]
	if !exist {
		val, existInKVStore, err := fms.kvStore.Get(TOPIC_SCHEMA, partition)
		if err != nil {
			return 0, err
		}
		if existInKVStore {
			sequenceValue, _ = strconv.ParseUint(string(val), 10, 0)
		} else {
			sequenceValue = uint64(0)
		}
	}
	fms.topicSequences[partition] = sequenceValue
	return sequenceValue, nil
}

// the the id to a new value
func (fms *DummyMessageStore) setId(partition string, id uint64) {
	fms.topicSequencesLock.Lock()
	defer fms.topicSequencesLock.Unlock()
	fms.topicSequences[partition] = id
}

func (fms *DummyMessageStore) startSequenceSync() {
	lastSyncValues := make(map[string]uint64)
	topicsToUpdate := []string{}

	shouldStop := false
	for !shouldStop {
		select {
		case <-time.After(fms.idSyncDuration):
		case <-fms.stopC:
			shouldStop = true
		}

		fms.topicSequencesLock.Lock()
		topicsToUpdate = topicsToUpdate[:0]
		for topic, seq := range fms.topicSequences {
			if lastSyncValues[topic] != seq {
				topicsToUpdate = append(topicsToUpdate, topic)
			}
		}
		fms.topicSequencesLock.Unlock()

		for _, topic := range topicsToUpdate {
			fms.topicSequencesLock.Lock()
			latestValue := fms.topicSequences[topic]
			fms.topicSequencesLock.Unlock()

			lastSyncValues[topic] = latestValue
			fms.kvStore.Put(TOPIC_SCHEMA, topic, []byte(strconv.FormatUint(latestValue, 10)))
		}
	}
	fms.stoppedC <- true
}

func (fms *DummyMessageStore) Check() error {
	return nil
}
