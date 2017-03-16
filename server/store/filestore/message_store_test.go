package filestore

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/cosminrentea/gobbler/server/store"

	"github.com/stretchr/testify/assert"
)

func Test_Fetch(t *testing.T) {
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "guble_message_store_test")
	//defer os.RemoveAll(dir)

	// when i store a message
	mStore := New(dir)
	a.NoError(mStore.Store("p1", uint64(1), []byte("aaaaaaaaaa")))
	a.NoError(mStore.Store("p1", uint64(2), []byte("bbbbbbbbbb")))
	a.NoError(mStore.Store("p2", uint64(1), []byte("1111111111")))
	a.NoError(mStore.Store("p2", uint64(2), []byte("2222222222")))

	testCases := []struct {
		description     string
		req             store.FetchRequest
		expectedResults []string
	}{
		{`match in partition 1`,
			store.FetchRequest{Partition: "p1", StartID: 2, Count: 1},
			[]string{"bbbbbbbbbb"},
		},
		{`match in partition 2`,
			store.FetchRequest{Partition: "p2", StartID: 2, Count: 1},
			[]string{"2222222222"},
		},
	}

	for _, testcase := range testCases {
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

func Test_MessageStore_Close(t *testing.T) {
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "guble_message_store_test")
	//defer os.RemoveAll(dir)

	// when i store a message
	store := New(dir)
	a.NoError(store.Store("p1", uint64(1), []byte("aaaaaaaaaa")))
	a.NoError(store.Store("p2", uint64(1), []byte("1111111111")))

	a.Equal(2, len(store.partitions))

	a.NoError(store.Stop())

	a.Equal(0, len(store.partitions))
}

func Test_MaxMessageId(t *testing.T) {
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "guble_message_store_test")
	//defer os.RemoveAll(dir)
	expectedMaxID := 2

	// when i store a message
	store := New(dir)
	a.NoError(store.Store("p1", uint64(1), []byte("aaaaaaaaaa")))
	a.NoError(store.Store("p1", uint64(expectedMaxID), []byte("bbbbbbbbbb")))

	maxID, err := store.MaxMessageID("p1")
	a.Nil(err, "No error should be received for partition p1")
	a.Equal(maxID, uint64(expectedMaxID), fmt.Sprintf("MaxId should be [%d]", expectedMaxID))
}

func Test_MaxMessageIdError(t *testing.T) {
	a := assert.New(t)
	store := New("/TestDir")

	_, err := store.MaxMessageID("p2")
	a.NotNil(err)
}

func Test_MessagePartitionReturningError(t *testing.T) {
	a := assert.New(t)

	store := New("/TestDir")
	_, err := store.Partition("p1")
	a.NotNil(err)
	fmt.Println(err)

	store2 := New("/")
	_, err2 := store2.Partition("p1")
	fmt.Println(err2)
}

func Test_FetchWithError(t *testing.T) {
	a := assert.New(t)
	mStore := New("/TestDir")

	chanCallBack := make(chan error, 1)
	aFetchRequest := store.FetchRequest{Partition: "p1", StartID: 2, Count: 1, ErrorC: chanCallBack}
	mStore.Fetch(&aFetchRequest)
	err := <-aFetchRequest.ErrorC
	a.NotNil(err)
}

func Test_StoreWithError(t *testing.T) {
	a := assert.New(t)
	mStore := New("/TestDir")

	err := mStore.Store("p1", uint64(1), []byte("124151qfas"))
	a.NotNil(err)
}

func Test_DoInTx(t *testing.T) {
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "guble_message_store_test")
	mStore := New(dir)
	a.NoError(mStore.Store("p1", uint64(1), []byte("aaaaaaaaaa")))

	err := mStore.DoInTx("p1", func(maxId uint64) error {
		return nil
	})
	a.Nil(err)
}

func Test_DoInTxError(t *testing.T) {
	a := assert.New(t)
	mStore := New("/TestDir")

	err := mStore.DoInTx("p2", nil)
	a.NotNil(err)
}

func Test_Check(t *testing.T) {
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "guble_message_store_test")
	mStore := New(dir)
	a.NoError(mStore.Store("p1", uint64(1), []byte("aaaaaaaaaa")))

	err := mStore.Check()
	a.Nil(err)
}

// func Test_Partitions(t *testing.T) {
// 	// Store multiple partitions then recreate the store and see if they are picked up
// 	a := assert.New(t)
// 	msg := []byte("test message data")

// 	dir, err := ioutil.TempDir("", "guble_message_store_test")
// 	a.NoError(err)
// 	store := New(dir)

// 	a.NoError(store.Store("p1", uint64(2), msg))
// 	a.NoError(store.Store("p2", uint64(2), msg))
// 	a.NoError(store.Store("p3", uint64(2), msg))

// 	store2 := New(dir)
// 	partitions, err := store2.Partitions()
// 	a.NoError(err)
// 	a.Equal(3, len(partitions))
// 	a.Equal("p1", partitions[0].Name)
// 	a.Equal("p2", partitions[1].Name)
// 	a.Equal("p3", partitions[2].Name)

// }
