package websocket

import (
	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/router"
	"github.com/cosminrentea/gobbler/server/store"
	"github.com/cosminrentea/gobbler/testutil"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"errors"
	"math"
	"testing"
	"time"
)

func Test_Receiver_error_handling_on_create(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	badArgs := []string{"", "20", "foo 20 20", "/foo 20 20 20", "/foo a", "/foo 20 b"}
	for _, arg := range badArgs {
		rec, _, _, _, err := aMockedReceiver(arg)
		a.Nil(rec, "Testing with: "+arg)
		a.Error(err, "Testing with: "+arg)
	}
}

func Test_Receiver_Fetch_Subscribe_Fetch_Subscribe(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	rec, msgChannel, routerMock, messageStore, err := aMockedReceiver("/foo 0")
	a.NoError(err)

	// fetch first, starting at 0
	fetchFirst1 := messageStore.EXPECT().Fetch(gomock.Any()).Do(func(r *store.FetchRequest) {
		go func() {
			a.Equal("foo", r.Partition)
			a.Equal(store.DirectionForward, r.Direction)
			a.Equal(uint64(0), r.StartID)
			a.Equal(int(math.MaxInt32), r.Count)

			r.StartC <- 2

			r.MessageC <- &store.FetchedMessage{ID: uint64(1), Message: []byte("fetch_first1-a")}
			r.MessageC <- &store.FetchedMessage{ID: uint64(2), Message: []byte("fetch_first1-b")}
			close(r.MessageC)
		}()
	})

	// there is a gap between fetched and max id
	messageID1 := messageStore.EXPECT().DoInTx(gomock.Any(), gomock.Any()).
		Do(func(partition string, callback func(maxMessageId uint64) error) {
			callback(uint64(3))
		}).Return(errUnreadMsgsAvailable)
	messageID1.After(fetchFirst1)

	// fetch again, starting at 3, because, there is still a gap
	fetchFirst2 := messageStore.EXPECT().Fetch(gomock.Any()).Do(func(r *store.FetchRequest) {
		go func() {
			a.Equal("foo", r.Partition)
			a.Equal(store.DirectionForward, r.Direction)
			a.Equal(uint64(3), r.StartID)
			a.Equal(int(math.MaxInt32), r.Count)

			r.StartC <- 1
			r.MessageC <- &store.FetchedMessage{ID: uint64(3), Message: []byte("fetch_first2-a")}
			close(r.MessageC)
		}()
	})
	fetchFirst2.After(messageID1)

	// the gap is closed
	messageID2 := messageStore.EXPECT().DoInTx(gomock.Any(), gomock.Any()).
		Do(func(partition string, callback func(maxMessageId uint64) error) {
			callback(uint64(3))
		})
	messageID2.After(fetchFirst2)

	// subscribe
	subscribe := routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(r *router.Route) {
		a.Equal(r.Path, protocol.Path("/foo"))
		r.Deliver(&protocol.Message{ID: uint64(4), Body: []byte("router-a"), Time: 1405544146}, true)
		r.Deliver(&protocol.Message{ID: uint64(5), Body: []byte("router-b"), Time: 1405544146}, true)
		r.Close() // emulate router close
	})
	subscribe.After(messageID2)

	// router closed, so we fetch again, starting at 6 (after meesages from subscribe)
	fetchAfter := messageStore.EXPECT().Fetch(gomock.Any()).Do(func(r *store.FetchRequest) {
		go func() {
			a.Equal(uint64(6), r.StartID)
			a.Equal(int(math.MaxInt32), r.Count)

			r.StartC <- 1
			r.MessageC <- &store.FetchedMessage{ID: uint64(6), Message: []byte("fetch_after-a")}
			close(r.MessageC)
		}()
	})
	fetchAfter.After(subscribe)

	// no gap
	messageID3 := messageStore.EXPECT().DoInTx(gomock.Any(), gomock.Any()).
		Do(func(partition string, callback func(maxMessageId uint64) error) {
			callback(uint64(6))
		})

	messageID3.After(fetchAfter)

	// subscribe and don't send messages,
	// so the client has to wait until we stop
	subscribe2 := routerMock.EXPECT().Subscribe(gomock.Any())
	subscribe2.After(messageID3)

	subscriptionLoopDone := make(chan bool)
	go func() {
		rec.subscriptionLoop()
		subscriptionLoopDone <- true
	}()

	expectMessages(a, msgChannel,
		"#"+protocol.SUCCESS_FETCH_START+" /foo 2",
		"fetch_first1-a",
		"fetch_first1-b",
		"#"+protocol.SUCCESS_FETCH_END+" /foo",
		"#"+protocol.SUCCESS_FETCH_START+" /foo 1",
		"fetch_first2-a",
		"#"+protocol.SUCCESS_FETCH_END+" /foo",
		"#"+protocol.SUCCESS_SUBSCRIBED_TO+" /foo",
		",4,,,,,1405544146,0\n\nrouter-a",
		",5,,,,,1405544146,0\n\nrouter-b",
		"#"+protocol.SUCCESS_FETCH_START+" /foo 1",
		"fetch_after-a",
		"#"+protocol.SUCCESS_FETCH_END+" /foo",
		"#"+protocol.SUCCESS_SUBSCRIBED_TO+" /foo",
	)

	time.Sleep(time.Millisecond)
	routerMock.EXPECT().Unsubscribe(gomock.Any())
	rec.Stop()

	expectMessages(a, msgChannel,
		"#"+protocol.SUCCESS_CANCELED+" /foo",
	)

	testutil.ExpectDone(a, subscriptionLoopDone)
}

func Test_Receiver_Fetch_Returns_Correct_Messages(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	rec, msgChannel, _, messageStore, err := aMockedReceiver("/foo 0 2")
	a.NoError(err)

	messages := []string{"The answer ", "is 42"}
	done := make(chan bool)
	messageStore.EXPECT().Fetch(gomock.Any()).Do(func(r *store.FetchRequest) {
		go func() {
			r.StartC <- len(messages)
			for i, m := range messages {
				r.MessageC <- &store.FetchedMessage{ID: uint64(i + 1), Message: []byte(m)}
			}
			close(r.MessageC)
			done <- true
		}()
	})

	fetchHasTerminated := make(chan bool)
	go func() {
		rec.fetchOnlyLoop()
		fetchHasTerminated <- true
	}()
	testutil.ExpectDone(a, done)

	expectMessages(a, msgChannel, "#"+protocol.SUCCESS_FETCH_START+" /foo 2")
	expectMessages(a, msgChannel, messages...)
	expectMessages(a, msgChannel, "#"+protocol.SUCCESS_FETCH_END+" /foo")

	testutil.ExpectDone(a, fetchHasTerminated)
	ctrl.Finish()
}

func Test_Receiver_Fetch_Produces_Correct_Fetch_Requests(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	testcases := []struct {
		desc   string
		arg    string
		maxID  int
		expect store.FetchRequest
	}{
		{desc: "simple forward fetch",
			arg:    "/foo 0 20",
			maxID:  -1,
			expect: store.FetchRequest{Partition: "foo", Direction: 1, StartID: uint64(0), Count: 20},
		},
		{desc: "forward fetch without bounds",
			arg:    "/foo 0",
			maxID:  -1,
			expect: store.FetchRequest{Partition: "foo", Direction: 1, StartID: uint64(0), Count: math.MaxInt32},
		},
		{desc: "backward fetch to top",
			arg:    "/foo -20",
			maxID:  42,
			expect: store.FetchRequest{Partition: "foo", Direction: -1, StartID: uint64(42), Count: 20},
		},
		{desc: "backward fetch with count",
			arg:    "/foo -1 10",
			maxID:  42,
			expect: store.FetchRequest{Partition: "foo", Direction: -1, StartID: uint64(42), Count: 10},
		},
	}

	for _, test := range testcases {
		rec, _, _, messageStore, err := aMockedReceiver(test.arg)

		a.NotNil(rec)
		a.NoError(err, test.desc)

		if test.maxID != -1 {
			messageStore.EXPECT().MaxMessageID(test.expect.Partition).
				Return(uint64(test.maxID), nil)
		}

		done := make(chan bool)
		messageStore.EXPECT().Fetch(gomock.Any()).Do(func(r *store.FetchRequest) {
			a.Equal(test.expect.Partition, r.Partition, test.desc)
			a.Equal(test.expect.Direction, r.Direction, test.desc)
			a.Equal(test.expect.StartID, r.StartID, test.desc)
			a.Equal(test.expect.Count, r.Count, test.desc)
			done <- true
		})

		go rec.fetchOnlyLoop()
		testutil.ExpectDone(a, done)
		rec.Stop()
	}
}

func Test_Receiver_Fetch_Sends_error_on_failure(t *testing.T) {
	a := assert.New(t)

	for _, arg := range []string{
		"/foo 0 2", // fetch only
		"/foo 0",   // fetch and subscribe
	} {
		ctrl := gomock.NewController(t)

		rec, msgChannel, _, messageStore, err := aMockedReceiver(arg)
		a.NoError(err)

		messageStore.EXPECT().Fetch(gomock.Any()).Do(func(r *store.FetchRequest) {
			go func() {
				r.ErrorC <- errors.New("expected test error")
			}()
		})

		rec.Start()
		expectMessages(a, msgChannel, "!error-server-internal expected test error")
		ctrl.Finish()
	}
}

func Test_Receiver_Fetch_Sends_error_on_failure_in_MaxMessageId(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	rec, msgChannel, _, messageStore, err := aMockedReceiver("/foo -2 2")
	a.NoError(err)

	messageStore.EXPECT().MaxMessageID("foo").
		Return(uint64(0), errors.New("expected test error"))

	rec.Start()

	expectMessages(a, msgChannel, "!error-server-internal expected test error")
}

//rec, sendChannel, router, messageStore, err := aMockedReceiver("+")
func aMockedReceiver(arg string) (*Receiver, chan []byte, *MockRouter, *MockMessageStore, error) {
	routerMock := NewMockRouter(testutil.MockCtrl)
	messageStore := NewMockMessageStore(testutil.MockCtrl)
	routerMock.EXPECT().MessageStore().Return(messageStore, nil).AnyTimes()
	sendChannel := make(chan []byte)
	cmd := &protocol.Cmd{
		Name: protocol.CmdReceive,
		Arg:  arg,
	}
	rec, err := NewReceiverFromCmd("any-appId", cmd, sendChannel, routerMock, "userId")
	return rec, sendChannel, routerMock, messageStore, err
}

func expectMessages(a *assert.Assertions, msgChannel chan []byte, message ...string) {
	for _, m := range message {
		select {
		case msg := <-msgChannel:
			a.Equal(m, string(msg))
		case <-time.After(time.Millisecond * 100):
			a.Fail("timeout: " + m)
			return
		}
	}
}
