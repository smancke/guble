package store

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
	"time"
)

func Test_Fetch(t *testing.T) {
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "message_store_test")
	//defer os.RemoveAll(dir)

	// when i store a message
	store := NewFileMessageStore(dir)
	a.NoError(store.Store("p1", uint64(1), []byte("aaaaaaaaaa")))
	a.NoError(store.Store("p1", uint64(2), []byte("bbbbbbbbbb")))
	a.NoError(store.Store("p2", uint64(1), []byte("1111111111")))
	a.NoError(store.Store("p2", uint64(2), []byte("2222222222")))

	testCases := []struct {
		description     string
		req             FetchRequest
		expectedResults []string
	}{
		{`match in partition 1`,
			FetchRequest{Partition: "p1", StartId: 2, Count: 1},
			[]string{"bbbbbbbbbb"},
		},
		{`match in partition 2`,
			FetchRequest{Partition: "p2", StartId: 2, Count: 1},
			[]string{"2222222222"},
		},
	}

	for _, testcase := range testCases {
		testcase.req.MessageC = make(chan []byte)
		testcase.req.ErrorCallback = make(chan error)

		messages := []string{}

		store.Fetch(testcase.req)
	loop:
		for {
			select {
			case msg, open := <-testcase.req.MessageC:
				if !open {
					break loop
				}
				messages = append(messages, string(msg))
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

func Test_MessageStore_Close(t *testing.T) {
	a := assert.New(t)
	dir, _ := ioutil.TempDir("", "message_store_test")
	//defer os.RemoveAll(dir)

	// when i store a message
	store := NewFileMessageStore(dir)
	a.NoError(store.Store("p1", uint64(1), []byte("aaaaaaaaaa")))
	a.NoError(store.Store("p2", uint64(1), []byte("1111111111")))

	a.Equal(2, len(store.partitions))

	a.NoError(store.Stop())

	a.Equal(0, len(store.partitions))
}
