package dummystore

import (
	"strconv"
	"testing"
	"time"

	"github.com/cosminrentea/gobbler/server/kvstore"
	"github.com/stretchr/testify/assert"
)

func Test_DummyMessageStore_IncreaseOnStore(t *testing.T) {
	a := assert.New(t)

	dms := New(kvstore.NewMemoryKVStore())

	a.Equal(uint64(0), fne(dms.MaxMessageID("partition")))
	a.NoError(dms.Store("partition", 1, []byte{}))
	a.NoError(dms.Store("partition", 2, []byte{}))
	a.Equal(uint64(2), fne(dms.MaxMessageID("partition")))
}

func Test_DummyMessageStore_ErrorOnWrongMessageId(t *testing.T) {
	a := assert.New(t)

	store := New(kvstore.NewMemoryKVStore())

	a.Equal(uint64(0), fne(store.MaxMessageID("partition")))
	a.Error(store.Store("partition", 42, []byte{}))
}

func Test_DummyMessageStore_InitIdsFromKvStore(t *testing.T) {
	a := assert.New(t)

	// given: a kv-store with some values, and a dummy-message-store based on it
	kvStore := kvstore.NewMemoryKVStore()
	kvStore.Put(topicSchema, "partition1", []byte("42"))
	kvStore.Put(topicSchema, "partition2", []byte("21"))
	dms := New(kvStore)

	// then
	a.Equal(uint64(42), fne(dms.MaxMessageID("partition1")))
	a.Equal(uint64(21), fne(dms.MaxMessageID("partition2")))
}

func Test_DummyMessageStore_SyncIds(t *testing.T) {
	a := assert.New(t)

	// given: a store which syncs every 1ms
	kvStore := kvstore.NewMemoryKVStore()
	dms := New(kvStore)
	dms.idSyncDuration = time.Millisecond

	a.Equal(uint64(0), fne(dms.MaxMessageID("partition")))
	_, exist, _ := kvStore.Get(topicSchema, "partition")
	a.False(exist)

	// and is started
	dms.Start()
	defer dms.Stop()

	// when: we set an id and wait longer than 1ms

	// lock&unlock mutex here, because normal invocation of setId() in the code is done while already protected by mutex
	dms.topicSequencesLock.Lock()
	dms.setID("partition", uint64(42))
	dms.topicSequencesLock.Unlock()
	time.Sleep(time.Millisecond * 4)

	// the value is synced to the kv store
	value, exist, _ := kvStore.Get(topicSchema, "partition")
	a.True(exist)
	a.Equal([]byte(strconv.FormatUint(uint64(42), 10)), value)
}

func Test_DummyMessageStore_SyncIdsOnStop(t *testing.T) {
	a := assert.New(t)

	// given: a store which syncs nearly never
	kvStore := kvstore.NewMemoryKVStore()
	dms := New(kvStore)
	dms.idSyncDuration = time.Hour

	// and is started
	dms.Start()

	// when: we set an id
	dms.topicSequencesLock.Lock()
	dms.setID("partition", uint64(42))
	dms.topicSequencesLock.Unlock()

	// then it is not synced after some wait
	time.Sleep(time.Millisecond * 2)
	_, exist, _ := kvStore.Get(topicSchema, "partition")
	a.False(exist)

	// but

	// when: we stop the store
	dms.Stop()

	// then: the the value is synced to the kv store
	value, exist, _ := kvStore.Get(topicSchema, "partition")
	a.True(exist)
	a.Equal([]byte(strconv.FormatUint(uint64(42), 10)), value)
}

func fne(args ...interface{}) interface{} {
	if args[1] != nil {
		panic(args[1])
	}
	return args[0]
}
