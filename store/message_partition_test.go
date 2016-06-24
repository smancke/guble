package store

import (
	"github.com/smancke/guble/testutil"

	"github.com/stretchr/testify/assert"

	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"
)

func Test_MessagePartition_scanFiles(t *testing.T) {
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	store, _ := NewMessagePartition(dir, "myMessages")

	a.NoError(ioutil.WriteFile(path.Join(dir, "myMessages-00000000000000420000.idx"), []byte{}, 0777))
	a.NoError(ioutil.WriteFile(path.Join(dir, "myMessages-00000000000000000000.idx"), []byte{}, 0777))
	a.NoError(ioutil.WriteFile(path.Join(dir, "myMessages-00000000000000010000.idx"), []byte{}, 0777))

	fileIds, err := store.scanFiles()
	a.NoError(err)
	a.Equal([]uint64{
		0,
		10000,
		420000,
	}, fileIds)
}

func Test_MessagePartition_correctIdAfterRestart(t *testing.T) {
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	store, _ := NewMessagePartition(dir, "myMessages")

	a.NoError(store.Store(uint64(1), []byte("aaaaaaaaaa")))
	a.NoError(store.Store(uint64(2), []byte("aaaaaaaaaa")))
	a.Equal(uint64(2), fne(store.MaxMessageId()))
	a.NoError(store.Close())

	newStore, err := NewMessagePartition(dir, "myMessages")
	a.NoError(err)
	a.Equal(uint64(2), fne(newStore.MaxMessageId()))
}

func TestCreateNextAppendFiles(t *testing.T) {
	a := assert.New(t)

	// given: a store
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	store, _ := NewMessagePartition(dir, "myMessages")

	// when i create append files
	a.NoError(store.createNextAppendFiles(uint64(424242)))

	a.Equal(uint64(420000), store.appendFirstId)
	a.Equal(uint64(429999), store.appendLastId)
	a.Equal(uint64(0x9), store.appendFileWritePosition)

	// and close the store
	a.NoError(store.Close())

	// then both files are empty
	msgs, errRead := ioutil.ReadFile(dir + "/myMessages-00000000000000420000.msg")
	a.NoError(errRead)
	a.Equal(9, len(msgs))
	a.Equal(MAGIC_NUMBER, msgs[0:8])
	a.Equal(byte(1), msgs[8])

	idx, errIdx := ioutil.ReadFile(dir + "/myMessages-00000000000000420000.idx")
	a.NoError(errIdx)
	a.Equal([]byte{}, idx)
}

func Test_Storing_Two_Messages_With_Append(t *testing.T) {
	a := assert.New(t)

	// given: a store
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)

	// when i store a message
	store, _ := NewMessagePartition(dir, "myMessages")
	a.NoError(store.Store(0x1, []byte("abc")))
	a.NoError(store.Close())

	// and I add another message with a new store
	store, _ = NewMessagePartition(dir, "myMessages")
	a.NoError(store.Store(0x2, []byte("defgh")))
	a.NoError(store.Close())

	// then both files as expected
	msgs, errRead := ioutil.ReadFile(dir + "/myMessages-00000000000000000000.msg")
	a.NoError(errRead)
	a.Equal(MAGIC_NUMBER, msgs[0:8])
	a.Equal(byte(1), msgs[8])
	a.Equal([]byte{
		3, 0, 0, 0, // len(abc) == 3
		1, 0, 0, 0, 0, 0, 0, 0, // id == 1
		'a', 'b', 'c',

		5, 0, 0, 0, // len(defgh) == 5
		2, 0, 0, 0, 0, 0, 0, 0, // id == 3
		'd', 'e', 'f', 'g', 'h', // defgh
	}, msgs[9:])

	idx, errIdx := ioutil.ReadFile(dir + "/myMessages-00000000000000000000.idx")
	a.NoError(errIdx)
	a.Equal([]byte{
		0, 0, 0, 0, 0, 0, 0, 0, // id 0: not set
		0, 0, 0, 0, // size == 0
		21, 0, 0, 0, 0, 0, 0, 0, // id 1: offset
		3, 0, 0, 0, // size == 0
		36, 0, 0, 0, 0, 0, 0, 0, // id 2: offset
		5, 0, 0, 0, // size == 5
	}, idx)
}

func Benchmark_Storing_HelloWorld_Messages(b *testing.B) {
	a := assert.New(b)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	store, _ := NewMessagePartition(dir, "myMessages")

	a.NoError(testutil.SyncStorage())
	b.ResetTimer()
	for i := 1; i <= b.N; i++ {
		a.NoError(store.Store(uint64(i), []byte("Hello World")))
	}
	a.NoError(store.Close())
	a.NoError(testutil.SyncStorage())
	b.StopTimer()
}

func Benchmark_Storing_1Kb_Messages(b *testing.B) {
	a := assert.New(b)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	store, _ := NewMessagePartition(dir, "myMessages")

	message := make([]byte, 1024)
	for i := range message {
		message[i] = 'a'
	}

	a.NoError(testutil.SyncStorage())
	b.ResetTimer()
	for i := 1; i <= b.N; i++ {
		a.NoError(store.Store(uint64(i), message))
	}
	a.NoError(store.Close())
	a.NoError(testutil.SyncStorage())
	b.StopTimer()
}

func Benchmark_Storing_1MB_Messages(b *testing.B) {
	a := assert.New(b)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	store, _ := NewMessagePartition(dir, "myMessages")

	message := make([]byte, 1024*1024)
	for i := range message {
		message[i] = 'a'
	}

	a.NoError(testutil.SyncStorage())
	b.ResetTimer()
	for i := 1; i <= b.N; i++ {
		a.NoError(store.Store(uint64(i), message))
	}
	a.NoError(store.Close())
	a.NoError(testutil.SyncStorage())
	b.StopTimer()
}

func TestFirstMessageIdForFile(t *testing.T) {
	a := assert.New(t)
	store := &MessagePartition{}
	a.Equal(uint64(0), store.firstMessageIdForFile(0))
	a.Equal(uint64(0), store.firstMessageIdForFile(1))
	a.Equal(uint64(0), store.firstMessageIdForFile(42))
	a.Equal(uint64(7680000), store.firstMessageIdForFile(7682334))
}

func Test_calculateFetchList(t *testing.T) {
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)

	// when i store a message
	store, _ := NewMessagePartition(dir, "myMessages")
	store.maxMessageId = uint64(2) // hack, for test setup
	a.NoError(store.Store(uint64(3), []byte("aaaaaaaaaa")))
	a.NoError(store.Store(uint64(4), []byte("bbbbbbbbbb")))
	store.maxMessageId = uint64(9) // hack, for test setup
	a.NoError(store.Store(uint64(10), []byte("cccccccccc")))
	store.maxMessageId = MESSAGES_PER_FILE - uint64(1) // hack, for test setup
	a.NoError(store.Store(MESSAGES_PER_FILE, []byte("1111111111")))
	store.maxMessageId = MESSAGES_PER_FILE + uint64(4) // hack, for test setup
	a.NoError(store.Store(MESSAGES_PER_FILE+uint64(5), []byte("2222222222")))
	defer a.NoError(store.Close())

	testCases := []struct {
		description     string
		req             FetchRequest
		expectedResults []fetchEntry
	}{
		{`direct match`,
			FetchRequest{StartId: 3, Direction: 0, Count: 1},
			[]fetchEntry{
				{3, uint64(0), 21, 10}, // messageId, fileId, offset, size
			},
		},
		{`direct match in second file`,
			FetchRequest{StartId: MESSAGES_PER_FILE, Direction: 0, Count: 1},
			[]fetchEntry{
				{MESSAGES_PER_FILE, MESSAGES_PER_FILE, 21, 10}, // messageId, fileId, offset, size
			},
		},
		{`next entry matches`,
			FetchRequest{StartId: 1, Direction: 0, Count: 1},
			[]fetchEntry{
				{3, uint64(0), 21, 10}, // messageId, fileId, offset, size
			},
		},
		{`entry before matches`,
			FetchRequest{StartId: 5, Direction: -1, Count: 1},
			[]fetchEntry{
				{4, uint64(0), 43, 10}, // messageId, fileId, offset, size
			},
		},
		{`backward, no match`,
			FetchRequest{StartId: 1, Direction: -1, Count: 1},
			[]fetchEntry{},
		},
		{`forward, no match (out of files)`,
			FetchRequest{StartId: 99999999999, Direction: 1, Count: 1},
			[]fetchEntry{},
		},
		{`forward, no match (after last id in last file)`,
			FetchRequest{StartId: MESSAGES_PER_FILE + uint64(8), Direction: 1, Count: 1},
			[]fetchEntry{},
		},
		/*
			{`forward, overlapping files`,
				FetchRequest{StartId: 9, Direction: 1, Count: 3},
				[]fetchEntry{
					fetchEntry{10, uint64(0), 65, 10},                                    // messageId, fileId, offset, size
					fetchEntry{MESSAGES_PER_FILE, MESSAGES_PER_FILE, 21, 10},             // messageId, fileId, offset, size
					fetchEntry{MESSAGES_PER_FILE + uint64(5), MESSAGES_PER_FILE, 43, 10}, // messageId, fileId, offset, size
				},
			},
			{`backward, overlapping files`,
				FetchRequest{StartId: MESSAGES_PER_FILE + uint64(100), Direction: -1, Count: 100},
				[]fetchEntry{
					fetchEntry{MESSAGES_PER_FILE + uint64(5), MESSAGES_PER_FILE, 43, 10}, // messageId, fileId, offset, size
					fetchEntry{MESSAGES_PER_FILE, MESSAGES_PER_FILE, 21, 10},             // messageId, fileId, offset, size
					fetchEntry{10, uint64(0), 65, 10},                                    // messageId, fileId, offset, size
					fetchEntry{4, uint64(0), 43, 10},                                     // messageId, fileId, offset, size
					fetchEntry{3, uint64(0), 21, 10},                                     // messageId, fileId, offset, size
				},
			},
		*/
	}

	for _, testcase := range testCases {
		testcase.req.Partition = "myMessages"
		fetchEntries, err := store.calculateFetchList(testcase.req)
		a.NoError(err, "Tescase: "+testcase.description)
		a.Equal(testcase.expectedResults, fetchEntries, "Tescase: "+testcase.description)
	}
}

func Test_Partition_Fetch(t *testing.T) {
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)

	// when i store a message
	store, _ := NewMessagePartition(dir, "myMessages")
	store.maxMessageId = uint64(2) // hack, for test setup
	a.NoError(store.Store(uint64(3), []byte("aaaaaaaaaa")))
	a.NoError(store.Store(uint64(4), []byte("bbbbbbbbbb")))
	store.maxMessageId = uint64(9) // hack, for test setup
	a.NoError(store.Store(uint64(10), []byte("cccccccccc")))
	store.maxMessageId = MESSAGES_PER_FILE - uint64(1) // hack, for test setup
	a.NoError(store.Store(MESSAGES_PER_FILE, []byte("1111111111")))
	store.maxMessageId = MESSAGES_PER_FILE + uint64(4) // hack, for test setup
	a.NoError(store.Store(MESSAGES_PER_FILE+uint64(5), []byte("2222222222")))
	defer a.NoError(store.Close())

	testCases := []struct {
		description     string
		req             FetchRequest
		expectedResults []string
	}{
		{`direct match`,
			FetchRequest{StartId: 3, Direction: 0, Count: 1},
			[]string{"aaaaaaaaaa"},
		},
		{`direct match in second file`,
			FetchRequest{StartId: MESSAGES_PER_FILE, Direction: 0, Count: 1},
			[]string{"1111111111"},
		},
		{`next entry matches`,
			FetchRequest{StartId: 1, Direction: 0, Count: 1},
			[]string{"aaaaaaaaaa"},
		},
		{`entry before matches`,
			FetchRequest{StartId: 5, Direction: -1, Count: 1},
			[]string{"bbbbbbbbbb"},
		},
		{`backward, no match`,
			FetchRequest{StartId: 1, Direction: -1, Count: 1},
			[]string{},
		},
		{`forward, no match (out of files)`,
			FetchRequest{StartId: 99999999999, Direction: 1, Count: 1},
			[]string{},
		},
		{`forward, no match (after last id in last file)`,
			FetchRequest{StartId: MESSAGES_PER_FILE + uint64(8), Direction: 1, Count: 1},
			[]string{},
		},
		/*
			{`forward, overlapping files`,
				FetchRequest{StartId: 9, Direction: 1, Count: 3},
				[]string{"cccccccccc", "1111111111", "2222222222"},
			},
			{`backward, overlapping files`,
				FetchRequest{StartId: MESSAGES_PER_FILE + uint64(100), Direction: -1, Count: 100},
				[]string{"2222222222", "1111111111", "cccccccccc", "bbbbbbbbbb", "aaaaaaaaaa"},
			},*/
	}
	for _, testcase := range testCases {
		testcase.req.Partition = "myMessages"
		testcase.req.MessageC = make(chan MessageAndId)
		testcase.req.ErrorCallback = make(chan error)
		testcase.req.StartCallback = make(chan int)

		messages := []string{}

		store.Fetch(testcase.req)

		select {
		case numberOfResults := <-testcase.req.StartCallback:
			a.Equal(len(testcase.expectedResults), numberOfResults)
		case <-time.After(time.Second):
			a.Fail("timeout")
			return
		}

	loop:
		for {
			select {
			case msg, open := <-testcase.req.MessageC:
				if !open {
					break loop
				}
				messages = append(messages, string(msg.Message))
			case err := <-testcase.req.ErrorCallback:
				a.Fail(err.Error())
				break loop
			case <-time.After(time.Second):
				a.Fail("timeout")
				return
			}
		}
		a.Equal(testcase.expectedResults, messages, "Tescase: "+testcase.description)
	}
}

func TestFilenameGeneration(t *testing.T) {
	a := assert.New(t)

	store := &MessagePartition{
		basedir: "/foo/bar/",
		name:    "myMessages",
	}

	a.Equal("/foo/bar/myMessages-00000000000000000042.msg", store.filenameByMessageId(42))
	a.Equal("/foo/bar/myMessages-00000000000000000042.idx", store.indexFilenameByMessageId(42))
	a.Equal("/foo/bar/myMessages-00000000000000000000.idx", store.indexFilenameByMessageId(0))
	a.Equal("/foo/bar/myMessages-00000000000000010000.idx", store.indexFilenameByMessageId(MESSAGES_PER_FILE))
}

func Test_firstMessageIdForFile(t *testing.T) {
	a := assert.New(t)
	store := &MessagePartition{}

	a.Equal(uint64(0), store.firstMessageIdForFile(0))
	a.Equal(uint64(0), store.firstMessageIdForFile(42))
	a.Equal(MESSAGES_PER_FILE, store.firstMessageIdForFile(MESSAGES_PER_FILE))
	a.Equal(MESSAGES_PER_FILE, store.firstMessageIdForFile(MESSAGES_PER_FILE+uint64(1)))
}

func Test_calculateMaxMessageIdFromIndex(t *testing.T) {
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)

	// when i store a message
	store, _ := NewMessagePartition(dir, "myMessages")
	a.NoError(store.Store(uint64(1), []byte("aaaaaaaaaa")))
	a.NoError(store.Store(uint64(2), []byte("bbbbbbbbbb")))

	maxMessageId, err := store.calculateMaxMessageIdFromIndex(uint64(0))
	a.NoError(err)
	a.Equal(uint64(2), maxMessageId)
}

func Test_MessagePartition_ErrorOnWrongMessageId(t *testing.T) {
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)

	// when i store a message
	store, _ := NewMessagePartition(dir, "myMessages")
	a.Error(store.Store(42, []byte{}))
}
