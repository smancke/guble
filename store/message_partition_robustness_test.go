package store

import (
	"github.com/smancke/guble/guble"

	"github.com/stretchr/testify/assert"

	"io/ioutil"
	"math"
	"os"
	"strconv"
	"testing"
	"time"
)

func Test_MessagePartition_foConcurrentWriteAndReads(t *testing.T) {
	//defer enableDebugForMethod()()
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "partition_store_test")
	defer os.RemoveAll(dir)

	store, _ := NewMessagePartition(dir, "myMessages")

	n := 1000 * 100

	writerDone := make(chan bool)
	readerDone := make(chan bool)
	go messagePartitionWriter(a, store, n, writerDone)
	go messagePartitionReader("reader1", a, store, n, readerDone)
	go messagePartitionReader("reader2", a, store, n, readerDone)
	go messagePartitionReader("reader3", a, store, n, readerDone)
	go messagePartitionReader("reader4", a, store, n, readerDone)
	go messagePartitionReader("reader5", a, store, n, readerDone)

	select {
	case <-writerDone:
	case <-time.After(time.Second * 15):
		a.Fail("writer timed out")
	}

	timeout := time.After(time.Second * 15)
	for i := 0; i < 5; i++ {
		select {
		case <-readerDone:
		case <-timeout:
			a.Fail("reader timed out")
		}
	}
}

func messagePartitionWriter(a *assert.Assertions, store *MessagePartition, n int, done chan bool) {
	for i := 1; i <= n; i++ {
		msg := []byte("Hello " + strconv.Itoa(i))
		a.NoError(store.Store(uint64(i), msg))
	}
	done <- true
}

func messagePartitionReader(name string, a *assert.Assertions, store *MessagePartition, n int, done chan bool) {
	lastReadMessage := 0
	for lastReadMessage < n {

		msgC := make(chan MessageAndId)
		errorC := make(chan error)

		guble.Debug("[%v] start fetching at: %v", name, lastReadMessage+1)
		store.Fetch(FetchRequest{
			Partition:     "myMessages",
			StartId:       uint64(lastReadMessage + 1),
			Direction:     1,
			Count:         math.MaxInt32,
			MessageC:      msgC,
			ErrorCallback: errorC,
		})

	fetch:

		for {
			select {
			case msgAndId, open := <-msgC:
				if !open {
					guble.Debug("[%v] stop fetching at %v", name, lastReadMessage)
					break fetch
				}
				a.Equal(lastReadMessage+1, int(msgAndId.Id))
				lastReadMessage = int(msgAndId.Id)
			case err := <-errorC:
				a.Fail("received error", err.Error())
				<-done
				return
			}
		}

	}
	guble.Debug("[%v] ready, got %v", name, lastReadMessage)
	done <- true
}
