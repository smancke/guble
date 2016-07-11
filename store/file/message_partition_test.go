package file

import (
	"fmt"
	"github.com/smancke/guble/store"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"sync"

	"github.com/stretchr/testify/assert"
)

func TestFileMessageStore_GenerateNextMsgId(t *testing.T) {
	a := assert.New(t)

	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	mStore, err := NewMessagePartition(dir, "node1")
	a.Nil(err)

	generatedIDs := make([]uint64, 0)
	lastId := uint64(0)

	for i := 0; i < 1000; i++ {
		id, _, err := mStore.generateNextMsgId(1)
		generatedIDs = append(generatedIDs, id)
		a.True(id > lastId, "Ids should be monotonic")
		lastId = id
		a.Nil(err)
	}
}

func TestFileMessageStore_GenerateNextMsgIdMultipleNodes(t *testing.T) {
	a := assert.New(t)

	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	mStore, err := NewMessagePartition(dir, "node1")
	a.Nil(err)

	dir2, _ := ioutil.TempDir("", "guble_message_partition_test2")
	defer os.RemoveAll(dir2)
	mStore2, err := NewMessagePartition(dir2, "node1")
	a.Nil(err)

	generatedIDs := make([]uint64, 0)
	lastId := uint64(0)

	for i := 0; i < 1000; i++ {
		id, _, err := mStore.generateNextMsgId(1)
		id2, _, err := mStore2.generateNextMsgId(2)
		a.True(id2 > id, "Ids should be monotonic")
		generatedIDs = append(generatedIDs, id)
		generatedIDs = append(generatedIDs, id2)
		time.Sleep(1 * time.Millisecond)
		a.True(id > lastId, "Ids should be monotonic")
		a.True(id2 > lastId, "Ids should be monotonic")
		lastId = id2
		a.Nil(err)
	}

	for i := 0; i < len(generatedIDs)-1; i++ {
		if generatedIDs[i] >= generatedIDs[i+1] {
			a.FailNow("Not Sorted")
		}
	}
}

func Test_MessagePartition_loadFiles(t *testing.T) {
	a := assert.New(t)
	// allow five messages per file
	MESSAGES_PER_FILE = uint64(5)

	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	mStore, _ := NewMessagePartition(dir, "myMessages")

	msgData := []byte("aaaaaaaaaa")             // 10 bytes message
	a.NoError(mStore.Store(uint64(3), msgData)) // stored offset 21, size: 10
	a.NoError(mStore.Store(uint64(4), msgData)) // stored offset 21+10+12=43

	a.NoError(mStore.Store(uint64(10), msgData)) // stored offset 43+22=65

	a.NoError(mStore.Store(uint64(9), msgData)) // stored offset 65+22=87
	a.NoError(mStore.Store(uint64(5), msgData)) // stored offset 87+22=109

	// here second file will start
	a.NoError(mStore.Store(uint64(8), msgData))  // stored offset 21
	a.NoError(mStore.Store(uint64(15), msgData)) // stored offset 43
	a.NoError(mStore.Store(uint64(13), msgData)) // stored offset 65

	a.NoError(mStore.Store(uint64(22), msgData)) // stored offset 87
	a.NoError(mStore.Store(uint64(23), msgData)) // stored offset 109

	// third file
	a.NoError(mStore.Store(uint64(24), msgData)) // stored offset 21
	a.NoError(mStore.Store(uint64(26), msgData)) // stored offset 43

	a.NoError(mStore.Store(uint64(30), msgData)) // stored offset 65
	a.NoError(mStore.Close())

	err := mStore.readIdxFiles()
	a.NoError(err)

	min, max, err := readMinMaxMsgIdFromIndexFile(path.Join(dir, "myMessages-00000000000000000000.idx"))
	a.Equal(uint64(3), min)
	a.Equal(uint64(10), max)
	a.NoError(err)

}

func Test_MessagePartition_correctIdAfterRestart(t *testing.T) {
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	mStore, _ := NewMessagePartition(dir, "myMessages")

	a.NoError(mStore.Store(uint64(1), []byte("aaaaaaaaaa")))
	a.NoError(mStore.Store(uint64(2), []byte("aaaaaaaaaa")))
	a.Equal(uint64(2), fne(mStore.MaxMessageId()))
	a.NoError(mStore.Close())

	newMStore, err := NewMessagePartition(dir, "myMessages")
	a.NoError(err)
	a.Equal(uint64(2), fne(newMStore.MaxMessageId()))
}

func Benchmark_Storing_HelloWorld_Messages(b *testing.B) {
	a := assert.New(b)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	mStore, _ := NewMessagePartition(dir, "myMessages")

	b.ResetTimer()
	for i := 1; i <= b.N; i++ {
		a.NoError(mStore.Store(uint64(i), []byte("Hello World")))
	}
	a.NoError(mStore.Close())
	b.StopTimer()
}

func Benchmark_Storing_1Kb_Messages(b *testing.B) {
	a := assert.New(b)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	mStore, _ := NewMessagePartition(dir, "myMessages")

	message := make([]byte, 1024)
	for i := range message {
		message[i] = 'a'
	}

	b.ResetTimer()
	for i := 1; i <= b.N; i++ {
		a.NoError(mStore.Store(uint64(i), message))
	}
	a.NoError(mStore.Close())
	b.StopTimer()
}

func Benchmark_Storing_1MB_Messages(b *testing.B) {
	a := assert.New(b)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	mStore, _ := NewMessagePartition(dir, "myMessages")

	message := make([]byte, 1024*1024)
	for i := range message {
		message[i] = 'a'
	}

	b.ResetTimer()
	for i := 1; i <= b.N; i++ {
		a.NoError(mStore.Store(uint64(i), message))
	}
	a.NoError(mStore.Close())
	b.StopTimer()
}

func Test_calculateFetchList(t *testing.T) {
	// allow five messages per file
	MESSAGES_PER_FILE = uint64(5)

	msgData := []byte("aaaaaaaaaa") // 10 bytes message

	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)

	mStore, _ := NewMessagePartition(dir, "myMessages")

	// File header: MAGIC_NUMBER + FILE_NUMBER_VERSION = 9 bytes in the file
	// For each stored message there is a 12 bytes write that contains the msgID and size

	a.NoError(mStore.Store(uint64(3), msgData)) // stored offset 21, size: 10
	a.NoError(mStore.Store(uint64(4), msgData)) // stored offset 21+10+12=43

	a.NoError(mStore.Store(uint64(10), msgData)) // stored offset 43+22=65

	a.NoError(mStore.Store(uint64(9), msgData)) // stored offset 65+22=87
	a.NoError(mStore.Store(uint64(5), msgData)) // stored offset 87+22=109

	// here second file will start
	a.NoError(mStore.Store(uint64(8), msgData))  // stored offset 21
	a.NoError(mStore.Store(uint64(15), msgData)) // stored offset 43
	a.NoError(mStore.Store(uint64(13), msgData)) // stored offset 65

	a.NoError(mStore.Store(uint64(22), msgData)) // stored offset 87
	a.NoError(mStore.Store(uint64(23), msgData)) // stored offset 109

	// third file
	a.NoError(mStore.Store(uint64(24), msgData)) // stored offset 21
	a.NoError(mStore.Store(uint64(26), msgData)) // stored offset 43

	a.NoError(mStore.Store(uint64(30), msgData)) // stored offset 65

	defer a.NoError(mStore.Close())

	testCases := []struct {
		description     string
		req             store.FetchRequest
		expectedResults IndexList
	}{
		{`direct match`,
			store.FetchRequest{StartID: 3, Direction: 0, Count: 1},
			IndexList{
				{3, uint64(21), 10, 0}, // messageId, offset, size, fileId
			},
		},
		{`direct match in second file`,
			store.FetchRequest{StartID: 8, Direction: 0, Count: 1},
			IndexList{
				{8, uint64(21), 10, 1}, // messageId, offset, size, fileId,
			},
		},
		{`direct match in second file, not first position`,
			store.FetchRequest{StartID: 13, Direction: 0, Count: 1},
			IndexList{
				{13, uint64(65), 10, 1}, // messageId, offset, size, fileId,
			},
		},
		// TODO this is caused by hasStartID() functions.This will be done when implementing the EndID logic
		// {`next entry matches`,
		// 	store.FetchRequest{StartID: 1, Direction: 0, Count: 1},
		// 	SortedIndexList{
		// 		{3, uint64(21), 10, 0}, // messageId, offset, size, fileId
		// 	},
		// },
		{`entry before matches`,
			store.FetchRequest{StartID: 5, Direction: -1, Count: 2},
			IndexList{
				{4, uint64(43), 10, 0},  // messageId, offset, size, fileId
				{5, uint64(109), 10, 0}, // messageId, offset, size, fileId
			},
		},
		{`backward, no match`,
			store.FetchRequest{StartID: 1, Direction: -1, Count: 1},
			IndexList{},
		},
		{`forward, no match (out of files)`,
			store.FetchRequest{StartID: 99999999999, Direction: 1, Count: 1},
			IndexList{},
		},
		{`forward, no match (after last id in last file)`,
			store.FetchRequest{StartID: 31, Direction: 1, Count: 1},
			IndexList{},
		},
		{`forward, overlapping files`,
			store.FetchRequest{StartID: 9, Direction: 1, Count: 3},
			IndexList{
				{9, uint64(87), 10, 0},  // messageId, offset, size, fileId
				{10, uint64(65), 10, 0}, // messageId, offset, size, fileId
				{13, uint64(65), 10, 1}, // messageId, offset, size, fileId
			},
		},
		{`backward, overlapping files`,
			store.FetchRequest{StartID: 26, Direction: -1, Count: 4},
			IndexList{
				// {15, uint64(43), 10, 1},  // messageId, offset, size, fileId
				{22, uint64(87), 10, 1},  // messageId, offset, size, fileId
				{23, uint64(109), 10, 1}, // messageId, offset, size, fileId
				{24, uint64(21), 10, 2},  // messageId, offset, size, fileId
				{26, uint64(43), 10, 2},  // messageId, offset, size, fileId
			},
		},
		{`forward, over more then 2 files`,
			store.FetchRequest{StartID: 5, Direction: 1, Count: 10},
			IndexList{
				{5, uint64(109), 10, 0},  // messageId, offset, size, fileId
				{8, uint64(21), 10, 1},   // messageId, offset, size, fileId
				{9, uint64(87), 10, 0},   // messageId, offset, size, fileId
				{10, uint64(65), 10, 0},  // messageId, offset, size, fileId
				{13, uint64(65), 10, 1},  // messageId, offset, size, fileId
				{15, uint64(43), 10, 1},  // messageId, offset, size, fileId
				{22, uint64(87), 10, 1},  // messageId, offset, size, fileId
				{23, uint64(109), 10, 1}, // messageId, offset, size, fileId
				{24, uint64(21), 10, 2},  // messageId, offset, size, fileId
				{26, uint64(43), 10, 2},  // messageId, offset, size, fileId
			},
		},
	}

	for _, testcase := range testCases {
		testcase.req.Partition = "myMessages"
		fetchEntries, err := mStore.calculateFetchList(&testcase.req)
		a.NoError(err, "Tescase: "+testcase.description)
		a.True(matchSortedList(t, testcase.expectedResults, *fetchEntries), "Tescase: "+testcase.description)
	}
}

func matchSortedList(t *testing.T, expected, actual IndexList) bool {
	if len(expected) != len(actual) {
		assert.Equal(t, len(expected), len(actual), "Invalid length")
		return false
	}

	for i, entry := range expected {
		a := actual[i]
		assert.Equal(t, *entry, *a)
		if entry.messageID != a.messageID ||
			entry.offset != a.offset ||
			entry.size != a.size ||
			entry.fileID != a.fileID {
			return false
		}
	}

	return true
}

func Test_Partition_Fetch(t *testing.T) {
	a := assert.New(t)
	// allow five messages per file
	MESSAGES_PER_FILE = uint64(5)

	msgData := []byte("aaaaaaaaaa")  // 10 bytes message
	msgData2 := []byte("1111111111") // 10 bytes message
	msgData3 := []byte("bbbbbbbbbb") // 10 bytes message

	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)

	mStore, _ := NewMessagePartition(dir, "myMessages")

	// File header: MAGIC_NUMBER + FILE_NUMBER_VERSION = 9 bytes in the file
	// For each stored message there is a 12 bytes write that contains the msgID and size

	a.NoError(mStore.Store(uint64(3), msgData)) // stored offset 21, size: 10
	a.NoError(mStore.Store(uint64(4), msgData)) // stored offset 21+10+12=43

	a.NoError(mStore.Store(uint64(10), msgData)) // stored offset 43+22=65

	a.NoError(mStore.Store(uint64(9), msgData2)) // stored offset 65+22=87
	a.NoError(mStore.Store(uint64(5), msgData3)) // stored offset 87+22=109

	// here second file will start
	a.NoError(mStore.Store(uint64(8), msgData2))  // stored offset 21
	a.NoError(mStore.Store(uint64(15), msgData))  // stored offset 43
	a.NoError(mStore.Store(uint64(13), msgData3)) // stored offset 65

	a.NoError(mStore.Store(uint64(22), msgData)) // stored offset 87
	a.NoError(mStore.Store(uint64(23), msgData)) // stored offset 109

	// third file
	a.NoError(mStore.Store(uint64(24), msgData)) // stored offset 21
	a.NoError(mStore.Store(uint64(26), msgData)) // stored offset 43

	a.NoError(mStore.Store(uint64(30), msgData)) // stored offset 65

	defer a.NoError(mStore.Close())

	testCases := []struct {
		description     string
		req             store.FetchRequest
		expectedResults []string
	}{
		{`direct match`,
			store.FetchRequest{StartID: 3, Direction: 0, Count: 1},
			[]string{"aaaaaaaaaa"},
		},
		{`direct match in second file`,
			store.FetchRequest{StartID: 8, Direction: 0, Count: 1},
			[]string{"1111111111"},
		},
		{`next entry matches`,
			store.FetchRequest{StartID: 13, Direction: 0, Count: 1},
			[]string{"bbbbbbbbbb"},
		},
		{`entry before matches`,
			store.FetchRequest{StartID: 5, Direction: -1, Count: 2},
			[]string{"aaaaaaaaaa", "bbbbbbbbbb"},
		},
		{`backward, no match`,
			store.FetchRequest{StartID: 1, Direction: -1, Count: 1},
			[]string{},
		},
		{`forward, no match (out of files)`,
			store.FetchRequest{StartID: 99999999999, Direction: 1, Count: 1},
			[]string{},
		},
		{`forward, no match (after last id in last file)`,
			store.FetchRequest{StartID: mStore.maxMessageId + uint64(8), Direction: 1, Count: 1},
			[]string{},
		},
		{`forward, overlapping files`,
			store.FetchRequest{StartID: 9, Direction: 1, Count: 3},
			[]string{"1111111111", "aaaaaaaaaa", "bbbbbbbbbb"},
		},
		{`forward, over more then 2 files`,
			store.FetchRequest{StartID: 5, Direction: 1, Count: 10},
			[]string{"bbbbbbbbbb", "1111111111", "1111111111", "aaaaaaaaaa", "bbbbbbbbbb", "aaaaaaaaaa", "aaaaaaaaaa", "aaaaaaaaaa", "aaaaaaaaaa", "aaaaaaaaaa"},
		},
		{`backward, overlapping files`,
			store.FetchRequest{StartID: 26, Direction: -1, Count: 4},
			[]string{"aaaaaaaaaa", "aaaaaaaaaa", "aaaaaaaaaa", "aaaaaaaaaa"},
		},
		{`backward, all messages`,
			store.FetchRequest{StartID: uint64(100), Direction: -1, Count: 100},
			[]string{"aaaaaaaaaa", "aaaaaaaaaa", "bbbbbbbbbb", "1111111111", "1111111111", "aaaaaaaaaa", "bbbbbbbbbb", "aaaaaaaaaa", "aaaaaaaaaa", "aaaaaaaaaa", "aaaaaaaaaa", "aaaaaaaaaa", "aaaaaaaaaa"},
		},
	}
	for _, testcase := range testCases {
		testcase.req.Partition = "myMessages"
		testcase.req.MessageC = make(chan store.MessageAndID)
		testcase.req.ErrorC = make(chan error)
		testcase.req.StartC = make(chan int)

		messages := []string{}

		mStore.Fetch(&testcase.req)

		select {
		case numberOfResults := <-testcase.req.StartC:
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
			case err := <-testcase.req.ErrorC:
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

	mStore := &MessagePartition{
		basedir:   "/foo/bar/",
		name:      "myMessages",
		mutex:     &sync.RWMutex{},
		fileCache: newCache(),
	}

	a.Equal("/foo/bar/myMessages-00000000000000000000.msg", mStore.composeMsgFilename())
	a.Equal("/foo/bar/myMessages-00000000000000000042.idx", mStore.composeIndexFilenameWithValue(42))
	a.Equal("/foo/bar/myMessages-00000000000000000000.idx", mStore.composeIndexFilenameWithValue(0))
	a.Equal(fmt.Sprintf("/foo/bar/myMessages-%020d.idx", MESSAGES_PER_FILE), mStore.composeIndexFilenameWithValue(MESSAGES_PER_FILE))
}

func fne(args ...interface{}) interface{} {
	if args[1] != nil {
		panic(args[1])
	}
	return args[0]
}
