package filestore

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/cosminrentea/gobbler/server/store"

	"errors"

	"github.com/stretchr/testify/assert"
)

func TestFileMessageStore_GenerateNextMsgId(t *testing.T) {
	a := assert.New(t)

	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	mStore, err := newMessagePartition(dir, "node1")
	a.Nil(err)

	var generatedIDs []uint64
	lastID := uint64(0)

	for i := 0; i < 1000; i++ {
		id, _, err := mStore.generateNextMsgID(1)
		generatedIDs = append(generatedIDs, id)
		a.True(id > lastID, "Ids should be monotonic")
		lastID = id
		a.Nil(err)
	}
}

func TestFileMessageStore_GenerateNextMsgIdMultipleNodes(t *testing.T) {
	a := assert.New(t)

	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	mStore, err := newMessagePartition(dir, "node1")
	a.Nil(err)

	dir2, _ := ioutil.TempDir("", "guble_message_partition_test2")
	defer os.RemoveAll(dir2)
	mStore2, err := newMessagePartition(dir2, "node1")
	a.Nil(err)

	var generatedIDs []uint64
	lastID := uint64(0)

	for i := 0; i < 1000; i++ {
		id, _, err := mStore.generateNextMsgID(1)
		id2, _, err := mStore2.generateNextMsgID(2)
		a.True(id2 > id, "Ids should be monotonic")
		generatedIDs = append(generatedIDs, id)
		generatedIDs = append(generatedIDs, id2)
		time.Sleep(1 * time.Millisecond)
		a.True(id > lastID, "Ids should be monotonic")
		a.True(id2 > lastID, "Ids should be monotonic")
		lastID = id2
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
	messagesPerFile = uint64(5)

	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	mStore, _ := newMessagePartition(dir, "myMessages")

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
	a.Equal(uint64(13), mStore.Count())

	a.NoError(mStore.Close())

	err := mStore.initialize()
	a.NoError(err)

	cEntry, err := readCacheEntryFromIdxFile(path.Join(dir, "myMessages-00000000000000000000.idx"))
	a.Equal(uint64(3), cEntry.min)
	a.Equal(uint64(10), cEntry.max)
	a.NoError(err)

	a.Equal(uint64(26), mStore.Count())

}

func Test_MessagePartition_correctIdAfterRestart(t *testing.T) {
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	mStore, _ := newMessagePartition(dir, "myMessages")

	a.NoError(mStore.Store(uint64(1), []byte("aaaaaaaaaa")))
	a.NoError(mStore.Store(uint64(2), []byte("aaaaaaaaaa")))
	a.Equal(uint64(2), mStore.MaxMessageID())
	a.NoError(mStore.Close())
	a.Equal(uint64(2), mStore.Count())

	newMStore, err := newMessagePartition(dir, "myMessages")
	a.NoError(err)
	a.Equal(uint64(2), newMStore.MaxMessageID())
	a.Equal(uint64(2), newMStore.Count())
}

func Benchmark_Storing_HelloWorld_Messages(b *testing.B) {
	a := assert.New(b)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)
	mStore, _ := newMessagePartition(dir, "myMessages")

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
	mStore, _ := newMessagePartition(dir, "myMessages")

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
	mStore, _ := newMessagePartition(dir, "myMessages")

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
	messagesPerFile = uint64(5)

	msgData := []byte("aaaaaaaaaa") // 10 bytes message

	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)

	mStore, _ := newMessagePartition(dir, "myMessages")

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
		expectedResults indexList
	}{
		{`direct match`,
			store.FetchRequest{StartID: 3, Direction: 0, Count: 1},
			indexList{
				items: []*index{{3, uint64(21), 10, 0}}, // messageId, offset, size, fileId
			},
		},
		{`direct match in second file`,
			store.FetchRequest{StartID: 8, Direction: 0, Count: 1},
			indexList{
				items: []*index{{8, uint64(21), 10, 1}}, // messageId, offset, size, fileId,
			},
		},
		{`direct match in second file, not first position`,
			store.FetchRequest{StartID: 13, Direction: 0, Count: 1},
			indexList{
				items: []*index{{13, uint64(65), 10, 1}}, // messageId, offset, size, fileId,
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
			indexList{
				items: []*index{
					{4, uint64(43), 10, 0},  // messageId, offset, size, fileId
					{5, uint64(109), 10, 0}, // messageId, offset, size, fileId
				},
			},
		},
		{`backward, no match`,
			store.FetchRequest{StartID: 1, Direction: -1, Count: 1},
			indexList{},
		},
		{`forward, no match (out of files)`,
			store.FetchRequest{StartID: 99999999999, Direction: 1, Count: 1},
			indexList{},
		},
		{`forward, no match (after last id in last file)`,
			store.FetchRequest{StartID: 31, Direction: 1, Count: 1},
			indexList{},
		},
		{`forward, overlapping files`,
			store.FetchRequest{StartID: 9, Direction: 1, Count: 3},
			indexList{
				items: []*index{
					{9, uint64(87), 10, 0},  // messageId, offset, size, fileId
					{10, uint64(65), 10, 0}, // messageId, offset, size, fileId
					{13, uint64(65), 10, 1}, // messageId, offset, size, fileId
				},
			},
		},
		{`backward, overlapping files`,
			store.FetchRequest{StartID: 26, Direction: -1, Count: 4},
			indexList{
				items: []*index{
					// {15, uint64(43), 10, 1},  // messageId, offset, size, fileId
					{22, uint64(87), 10, 1},  // messageId, offset, size, fileId
					{23, uint64(109), 10, 1}, // messageId, offset, size, fileId
					{24, uint64(21), 10, 2},  // messageId, offset, size, fileId
					{26, uint64(43), 10, 2},  // messageId, offset, size, fileId
				},
			},
		},
		{`forward, over more then 2 files`,
			store.FetchRequest{StartID: 5, Direction: 1, Count: 10},
			indexList{
				items: []*index{
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
		},
	}

	for _, testcase := range testCases {
		testcase.req.Partition = "myMessages"
		fetchEntries, err := mStore.calculateFetchList(&testcase.req)
		a.NoError(err, "Tescase: "+testcase.description)
		a.True(matchSortedList(t, testcase.expectedResults, *fetchEntries), "Tescase: "+testcase.description)
	}
}

func matchSortedList(t *testing.T, expected, actual indexList) bool {
	if !assert.Equal(t, expected.len(), actual.len(), "Invalid length") {
		return false
	}

	err := expected.mapWithPredicate(func(elem *index, i int) error {
		a := actual.get(i)
		assert.Equal(t, *elem, *a)
		if elem.id != a.id ||
			elem.offset != a.offset ||
			elem.size != a.size ||
			elem.fileID != a.fileID {
			return errors.New("Element not equal!")
		}
		return nil
	})
	return assert.NoError(t, err)
}

func Test_Partition_Fetch(t *testing.T) {
	a := assert.New(t)

	// allow five messages per file
	messagesPerFile = uint64(5)

	msgData := []byte("1111111111")  // 10 bytes message
	msgData2 := []byte("2222222222") // 10 bytes message
	msgData3 := []byte("3333333333") // 10 bytes message

	dir, _ := ioutil.TempDir("", "guble_message_partition_test")
	defer os.RemoveAll(dir)

	mStore, _ := newMessagePartition(dir, "myMessages")

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
			[]string{"1111111111"},
		},
		{`direct match in second file`,
			store.FetchRequest{StartID: 8, Direction: 0, Count: 1},
			[]string{"2222222222"},
		},
		{`next entry matches`,
			store.FetchRequest{StartID: 13, Direction: 0, Count: 1},
			[]string{"3333333333"},
		},
		{`entry before matches`,
			store.FetchRequest{StartID: 5, Direction: -1, Count: 2},
			[]string{"1111111111", "3333333333"},
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
			store.FetchRequest{StartID: mStore.maxMessageID + uint64(8), Direction: 1, Count: 1},
			[]string{},
		},
		{`forward, overlapping files`,
			store.FetchRequest{StartID: 9, Direction: 1, Count: 3},
			[]string{"2222222222", "1111111111", "3333333333"},
		},
		{`forward, over more then 2 files`,
			store.FetchRequest{StartID: 5, Direction: 1, Count: 10},
			[]string{"3333333333", "2222222222", "2222222222", "1111111111", "3333333333", "1111111111", "1111111111", "1111111111", "1111111111", "1111111111"},
		},
		{`backward, overlapping files`,
			store.FetchRequest{StartID: 26, Direction: -1, Count: 4},
			[]string{"1111111111", "1111111111", "1111111111", "1111111111"},
		},
		{`backward, all messages`,
			store.FetchRequest{StartID: uint64(100), Direction: -1, Count: 100},
			[]string{"1111111111", "1111111111", "3333333333", "2222222222", "2222222222", "1111111111", "3333333333", "1111111111", "1111111111", "1111111111", "1111111111", "1111111111", "1111111111"},
		},
	}
	for _, testcase := range testCases {
		testcase.req.Partition = "myMessages"
		testcase.req.MessageC = make(chan *store.FetchedMessage)
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

	mStore := &messagePartition{
		basedir:   "/foo/bar/",
		name:      "myMessages",
		fileCache: newCache(),
	}

	a.Equal("/foo/bar/myMessages-00000000000000000000.msg", mStore.composeMsgFilenameForPosition(uint64(mStore.fileCache.length())))
	a.Equal("/foo/bar/myMessages-00000000000000000042.idx", mStore.composeIdxFilenameForPosition(42))
	a.Equal("/foo/bar/myMessages-00000000000000000000.idx", mStore.composeIdxFilenameForPosition(0))
	a.Equal(fmt.Sprintf("/foo/bar/myMessages-%020d.idx", messagesPerFile), mStore.composeIdxFilenameForPosition(messagesPerFile))
}
