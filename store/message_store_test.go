package store

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

func TestCreateNextAppendFiles(t *testing.T) {
	a := assert.New(t)

	// given: a store
	dir, _ := ioutil.TempDir("", "message_store_test")
	defer os.RemoveAll(dir)
	store := NewFileMessageStore(dir, "myMessages")

	// when i create append files
	a.NoError(store.createNextAppendFiles(uint64(424242)))

	a.Equal(uint64(420000), store.appendFirstId)
	a.Equal(uint64(429999), store.appendLastId)
	a.Equal(uint64(0), store.appendFileWritePosition)

	// and close the store
	a.NoError(store.Close())

	// then both files are empty
	msgs, errRead := ioutil.ReadFile(dir + "/myMessages-00000000000000420000.msg")
	a.NoError(errRead)
	a.Equal([]byte{}, msgs)

	idx, errIdx := ioutil.ReadFile(dir + "/myMessages-00000000000000420000.msg")
	a.NoError(errIdx)
	a.Equal([]byte{}, idx)
}

func TestStoringSomeMessages(t *testing.T) {
	a := assert.New(t)

	// given: a store
	dir, _ := ioutil.TempDir("", "message_store_test")
	defer os.RemoveAll(dir)
	store := NewFileMessageStore(dir, "myMessages")

	// when i store some messages
	a.NoError(store.Store(0x1, []byte("abc")))
	a.NoError(store.Store(0x2, []byte("defgh")))
	a.NoError(store.Close())

	// then both files as expected
	msgs, errRead := ioutil.ReadFile(dir + "/myMessages-00000000000000000000.msg")
	a.NoError(errRead)
	a.Equal([]byte{
		3, 0, 0, 0, // len(abc) == 3
		1, 0, 0, 0, 0, 0, 0, 0, // id == 1
		'a', 'b', 'c',

		5, 0, 0, 0, // len(defgh) == 5
		2, 0, 0, 0, 0, 0, 0, 0, // id == 2
		'd', 'e', 'f', 'g', 'h', // defgh
	}, msgs)

	idx, errIdx := ioutil.ReadFile(dir + "/myMessages-00000000000000000000.idx")
	a.NoError(errIdx)
	a.Equal([]byte{
		1, 0, 0, 0, 0, 0, 0, 0, // id == 1
		12, 0, 0, 0, 0, 0, 0, 0, // offset of payload message
		0, // ! deleted

		2, 0, 0, 0, 0, 0, 0, 0, // id == 2
		27, 0, 0, 0, 0, 0, 0, 0, // 4 + 8 + 3
		0, // ! deleted
	}, idx)
}

func Benchmark_Storing_HelloWorld_Messages(b *testing.B) {
	a := assert.New(b)
	dir, _ := ioutil.TempDir("", "message_store_test")
	defer os.RemoveAll(dir)
	store := NewFileMessageStore(dir, "myMessages")

	b.ResetTimer()
	for i := 0; i <= b.N; i++ {
		a.NoError(store.Store(uint64(i), []byte("Hello World")))
	}
	a.NoError(store.Close())
	b.StopTimer()
}

func Benchmark_Storing_1Kb_Messages(b *testing.B) {
	a := assert.New(b)
	dir, _ := ioutil.TempDir("", "message_store_test")
	defer os.RemoveAll(dir)
	store := NewFileMessageStore(dir, "myMessages")

	message := make([]byte, 1024)
	for i, _ := range message {
		message[i] = 'a'
	}

	b.ResetTimer()
	for i := 0; i <= b.N; i++ {
		a.NoError(store.Store(uint64(i), message))
	}
	a.NoError(store.Close())
	b.StopTimer()
}

func Benchmark_Storing_1MB_Messages(b *testing.B) {
	a := assert.New(b)
	dir, _ := ioutil.TempDir("", "message_store_test")
	defer os.RemoveAll(dir)
	store := NewFileMessageStore(dir, "myMessages")

	message := make([]byte, 1024*1024)
	for i, _ := range message {
		message[i] = 'a'
	}

	b.ResetTimer()
	for i := 0; i <= b.N; i++ {
		a.NoError(store.Store(uint64(i), message))
	}
	a.NoError(store.Close())
	b.StopTimer()
}

func TestReadAndWriteMindex(t *testing.T) {
	a := assert.New(t)

	buff := make([]byte, INDEX_SIZE_BYTES, INDEX_SIZE_BYTES)

	// deleted == true
	mi := &mIndex{42, 10340, true}
	mi.writeTo(buff)
	a.Equal(mi, readMIndex(buff))
	a.Equal(mi, readMIndex(mi.Bytes()))

	// deleted == false
	mi = &mIndex{42, 10340, false}
	mi.writeTo(buff)
	a.Equal(mi, readMIndex(buff))
	a.Equal(mi, readMIndex(mi.Bytes()))
}

func TestFirstMessageIdForFile(t *testing.T) {
	a := assert.New(t)
	store := &FileMessageStore{}
	a.Equal(uint64(0), store.firstMessageIdForFile(0))
	a.Equal(uint64(0), store.firstMessageIdForFile(1))
	a.Equal(uint64(0), store.firstMessageIdForFile(42))
	a.Equal(uint64(7680000), store.firstMessageIdForFile(7682334))
}

func TestFilenameGeneration(t *testing.T) {
	a := assert.New(t)

	store := &FileMessageStore{
		basedir: "/foo/bar/",
		name:    "myMessages",
	}

	a.Equal("/foo/bar/myMessages-00000000000000000042.msg", store.filenameByMessageId(42))
	a.Equal("/foo/bar/myMessages-00000000000000000042.idx", store.indexFilenameByMessageId(42))
}
