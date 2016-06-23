package store

import (
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
	"time"
)

func Test_DummyMessageStore_IncreaseOnStore(t *testing.T) {
	a := assert.New(t)

	store := NewDummyMessageStore(NewMemoryKVStore())

	a.Equal(uint64(0), fne(store.MaxMessageId("partition")))
	a.NoError(store.Store("partition", 1, []byte{}))
	a.NoError(store.Store("partition", 2, []byte{}))
	a.Equal(uint64(2), fne(store.MaxMessageId("partition")))
}

func Test_DummyMessageStore_ErrorOnWrongMessageId(t *testing.T) {
	a := assert.New(t)

	store := NewDummyMessageStore(NewMemoryKVStore())

	a.Equal(uint64(0), fne(store.MaxMessageId("partition")))
	a.Error(store.Store("partition", 42, []byte{}))
}

func Test_DummyMessageStore_InitIdsFromKvStore(t *testing.T) {
	a := assert.New(t)

	// given: as kv store with some values
	kvStore := NewMemoryKVStore()
	kvStore.Put(topicSchema, "partition1", []byte("42"))
	kvStore.Put(topicSchema, "partition2", []byte("43"))
	store := NewDummyMessageStore(kvStore)

	// then
	a.Equal(uint64(42), fne(store.MaxMessageId("partition1")))
	a.Equal(uint64(43), fne(store.MaxMessageId("partition2")))
}

func Test_DummyMessageStore_SyncIds(t *testing.T) {
	a := assert.New(t)

	// given: a store which syncs every 1ms
	kvStore := NewMemoryKVStore()
	store := NewDummyMessageStore(kvStore)
	store.idSyncDuration = time.Millisecond

	a.Equal(uint64(0), fne(store.MaxMessageId("partition")))
	_, exist, _ := kvStore.Get(topicSchema, "partition")
	a.False(exist)

	// and is started
	store.Start()
	defer store.Stop()

	// when: we set an id and wait for 4ms
	// Lock/unlock mutex here, because normal invocation of setId() in the code is done while already protected by mutex
	store.topicSequencesLock.Lock()
	store.setId("partition", uint64(42))
	store.topicSequencesLock.Unlock()
	time.Sleep(time.Millisecond * 4)

	// the value is synced to the kv store
	value, exist, _ := kvStore.Get(topicSchema, "partition")
	a.True(exist)
	a.Equal([]byte(strconv.FormatUint(uint64(42), 10)), value)
}

func Test_DummyMessageStore_SyncIdsOnStop(t *testing.T) {
	a := assert.New(t)

	// given: as store which synces nearly never
	kvStore := NewMemoryKVStore()
	store := NewDummyMessageStore(kvStore)
	store.idSyncDuration = time.Hour

	// and is started
	store.Start()

	// when: we set an id
	store.setId("partition", uint64(42))

	// then it is not synced after some wait
	time.Sleep(time.Millisecond * 2)
	_, exist, _ := kvStore.Get(topicSchema, "partition")
	a.False(exist)

	// but

	// when: we stop the store
	store.Stop()

	// then: the the value is synced to the kv store
	value, exist, _ := kvStore.Get(topicSchema, "partition")
	a.True(exist)
	a.Equal([]byte(strconv.FormatUint(uint64(42), 10)), value)
}

func Test_CheckDummyStore(t *testing.T) {
	a := assert.New(t)
	store := NewDummyMessageStore(NewMemoryKVStore())

	err := store.Check()
	a.Nil(err)
}

func fne(args ...interface{}) interface{} {
	if args[1] != nil {
		panic(args[1])
	}
	return args[0]
}
