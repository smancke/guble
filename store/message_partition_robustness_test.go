package store

import (
	"github.com/smancke/guble/protocol"

	"github.com/stretchr/testify/assert"

	"io/ioutil"
	"math"
	"os"
	"strconv"
	"testing"
	"time"
)

func Test_MessagePartition_forConcurrentWriteAndReads(t *testing.T) {
	//defer enableDebugForMethod()()
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "partition_store_test")
	defer os.RemoveAll(dir)

	store, _ := NewMessagePartition(dir, "myMessages")

	n := 1000 * 100
	nReaders := 5

	writerDone := make(chan bool)
	go messagePartitionWriter(a, store, n, writerDone)

	readerDone := make(chan bool)
	for i := 1; i <= nReaders; i++ {
		go messagePartitionReader("reader"+strconv.Itoa(i), a, store, n, readerDone)
	}

	select {
	case <-writerDone:
	case <-time.After(time.Second * 15):
		a.Fail("writer timed out")
	}

	timeout := time.After(time.Second * 15)
	for i := 0; i < nReaders; i++ {
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

		msgC := make(chan MessageAndID)
		errorC := make(chan error)

		protocol.Debug("[%v] start fetching at: %v", name, lastReadMessage+1)
		store.Fetch(FetchRequest{
			Partition: "myMessages",
			StartID:   uint64(lastReadMessage + 1),
			Direction: 1,
			Count:     math.MaxInt32,
			MessageC:  msgC,
			ErrorC:    errorC,
			StartC:    make(chan int, 1),
		})

	fetch:

		for {
			select {
			case msgAndId, open := <-msgC:
				if !open {
					protocol.Debug("[%v] stop fetching at %v", name, lastReadMessage)
					break fetch
				}
				a.Equal(lastReadMessage+1, int(msgAndId.ID))
				lastReadMessage = int(msgAndId.ID)
			case err := <-errorC:
				a.Fail("received error", err.Error())
				<-done
				return
			}
		}

	}
	protocol.Debug("[%v] ready, got %v", name, lastReadMessage)
	done <- true
}
