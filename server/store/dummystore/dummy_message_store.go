package dummystore

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/kvstore"
	"github.com/cosminrentea/gobbler/server/store"
)

const topicSchema = "topic_sequence"

// DummyMessageStore is a minimal implementation of the MessageStore interface.
// Everything it does is storing the message ids in the key value store to
// ensure a monotonic incremented id.
// It is intended for testing and demo purpose, as well as dummy for services without persistence.
// TODO: implement a simple logic to preserve the last N messages
type DummyMessageStore struct {
	topicSequences     map[string]uint64
	topicSequencesLock sync.RWMutex
	kvStore            kvstore.KVStore
	isSyncStarted      bool

	stopC    chan bool // used to send the stop request to the syc goroutine
	stoppedC chan bool // answer from the syc goroutine, when it is stopped

	idSyncDuration time.Duration
}

// New returns a new DummyMessageStore.
func New(kvStore kvstore.KVStore) *DummyMessageStore {
	return &DummyMessageStore{
		topicSequences: make(map[string]uint64),
		kvStore:        kvStore,
		idSyncDuration: time.Millisecond * 100,
		stopC:          make(chan bool, 1),
		stoppedC:       make(chan bool, 1),
	}
}

// Start the DummyMessageStore.
func (dms *DummyMessageStore) Start() error {
	go dms.startSequenceSync()
	dms.isSyncStarted = true
	return nil
}

// Stop the DummyMessageStore.
func (dms *DummyMessageStore) Stop() error {
	if !dms.isSyncStarted {
		return nil
	}
	dms.stopC <- true
	<-dms.stoppedC
	return nil
}

// StoreMessage is a part of the `store.MessageStore` implementation.
func (dms *DummyMessageStore) StoreMessage(message *protocol.Message, nodeID uint8) (int, error) {
	partitionName := message.Path.Partition()
	nextID, ts, err := dms.GenerateNextMsgID(partitionName, 0)
	if err != nil {
		return 0, err
	}
	message.ID = nextID
	message.Time = ts
	message.NodeID = nodeID
	data := message.Bytes()
	if err := dms.Store(partitionName, nextID, data); err != nil {
		return 0, err
	}
	return len(data), nil
}

// Store is a part of the `store.MessageStore` implementation.
func (dms *DummyMessageStore) Store(partition string, msgID uint64, msg []byte) error {
	dms.topicSequencesLock.Lock()
	defer dms.topicSequencesLock.Unlock()
	return dms.store(partition, msgID, msg)
}

func (dms *DummyMessageStore) store(partition string, msgId uint64, msg []byte) error {
	maxID, err := dms.maxMessageID(partition)
	if err != nil {
		return err
	}
	if msgId > 1+maxID {
		return fmt.Errorf("DummyMessageStore: Invalid message id for partition %v. Next id should be %v, but was %q",
			partition, 1+maxID, msgId)
	}
	dms.setID(partition, msgId)
	return nil
}

// Fetch does nothing in this dummy implementation.
// It is a part of the `store.MessageStore` implementation.
func (dms *DummyMessageStore) Fetch(req *store.FetchRequest) {
}

// MaxMessageID is a part of the `store.MessageStore` implementation.
func (dms *DummyMessageStore) MaxMessageID(partition string) (uint64, error) {
	dms.topicSequencesLock.Lock()
	defer dms.topicSequencesLock.Unlock()
	return dms.maxMessageID(partition)
}

// DoInTx is a part of the `store.MessageStore` implementation.
func (dms *DummyMessageStore) DoInTx(partition string, fnToExecute func(maxMessageId uint64) error) error {
	dms.topicSequencesLock.Lock()
	defer dms.topicSequencesLock.Unlock()
	maxID, err := dms.maxMessageID(partition)
	if err != nil {
		return err
	}
	return fnToExecute(maxID)
}

// GenerateNextMsgID is a part of the `store.MessageStore` implementation.
func (dms *DummyMessageStore) GenerateNextMsgID(partitionName string, nodeID uint8) (uint64, int64, error) {
	dms.topicSequencesLock.Lock()
	defer dms.topicSequencesLock.Unlock()
	ts := time.Now().Unix()
	max, err := dms.maxMessageID(partitionName)
	if err != nil {
		return 0, 0, err
	}
	next := max + 1
	dms.setID(partitionName, next)
	return next, ts, nil
}

func (dms *DummyMessageStore) maxMessageID(partition string) (uint64, error) {
	sequenceValue, exist := dms.topicSequences[partition]
	if !exist {
		val, existInKVStore, err := dms.kvStore.Get(topicSchema, partition)
		if err != nil {
			return 0, err
		}
		if existInKVStore {
			sequenceValue, err = strconv.ParseUint(string(val), 10, 0)
			if err != nil {
				return 0, err
			}
		} else {
			sequenceValue = uint64(0)
		}
	}
	dms.topicSequences[partition] = sequenceValue
	return sequenceValue, nil
}

// the id to a new value
func (dms *DummyMessageStore) setID(partition string, id uint64) {
	dms.topicSequences[partition] = id
}

func (dms *DummyMessageStore) startSequenceSync() {
	lastSyncValues := make(map[string]uint64)
	topicsToUpdate := []string{}

	shouldStop := false
	for !shouldStop {
		select {
		case <-time.After(dms.idSyncDuration):
		case <-dms.stopC:
			shouldStop = true
		}

		dms.topicSequencesLock.Lock()
		topicsToUpdate = topicsToUpdate[:0]
		for topic, seq := range dms.topicSequences {
			if lastSyncValues[topic] != seq {
				topicsToUpdate = append(topicsToUpdate, topic)
			}
		}
		dms.topicSequencesLock.Unlock()

		for _, topic := range topicsToUpdate {
			dms.topicSequencesLock.Lock()
			latestValue := dms.topicSequences[topic]
			dms.topicSequencesLock.Unlock()

			lastSyncValues[topic] = latestValue
			dms.kvStore.Put(topicSchema, topic, []byte(strconv.FormatUint(latestValue, 10)))
		}
	}
	dms.stoppedC <- true
}

func (dms *DummyMessageStore) Check() error {
	return nil
}

func (dms *DummyMessageStore) Partition(name string) (store.MessagePartition, error) {
	return nil, nil
}

func (dms *DummyMessageStore) Partitions() ([]store.MessagePartition, error) {
	return nil, nil
}
