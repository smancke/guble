package store

import (
	"github.com/stretchr/testify/assert"

	log "github.com/Sirupsen/logrus"
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

		msgC := make(chan MessageAndId)
		errorC := make(chan error)

		log.WithFields(log.Fields{
			"module":      "testing",
			"name":        name,
			"lastReadMsg": lastReadMessage + 1,
		}).Debug("Start fetching at")

		store.Fetch(FetchRequest{
			Partition:     "myMessages",
			StartId:       uint64(lastReadMessage + 1),
			Direction:     1,
			Count:         math.MaxInt32,
			MessageC:      msgC,
			ErrorCallback: errorC,
			StartCallback: make(chan int, 1),
		})

	fetch:

		for {
			select {
			case msgAndId, open := <-msgC:
				if !open {

					log.WithFields(log.Fields{
						"module":      "testing",
						"name":        name,
						"lastReadMsg": lastReadMessage,
					}).Debug("Stop fetching at")

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
	log.WithFields(log.Fields{
		"module":      "testing",
		"name":        name,
		"lastReadMsg": lastReadMessage,
	}).Debug("Ready got id")

	done <- true
}
