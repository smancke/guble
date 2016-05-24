package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"errors"
	"math"
	"testing"
	"time"
)

func Test_Receiver_error_handling_on_create(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	badArgs := []string{"", "20", "foo 20 20", "/foo 20 20 20", "/foo a", "/foo 20 b"}
	for _, arg := range badArgs {
		rec, _, _, _, err := aMockedReceiver(arg)
		a.Nil(rec, "Testing with: "+arg)
		a.Error(err, "Testing with: "+arg)
	}
}

func Test_Receiver_Fetch_Subscribe_Fetch_Subscribe(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	rec, msgChannel, routerMock, messageStore, err := aMockedReceiver("/foo 0")
	a.NoError(err)

	// fetch first, starting at 0
	fetch_first1 := messageStore.EXPECT().Fetch(gomock.Any()).Do(func(r store.FetchRequest) {
		go func() {
			a.Equal("foo", r.Partition)
			a.Equal(1, r.Direction)
			a.Equal(uint64(0), r.StartId)
			a.Equal(int(math.MaxInt32), r.Count)

			r.StartCallback <- 2

			r.MessageC <- store.MessageAndId{Id: uint64(1), Message: []byte("fetch_first1-a")}
			r.MessageC <- store.MessageAndId{Id: uint64(2), Message: []byte("fetch_first1-b")}
			close(r.MessageC)
		}()
	})

	// there is a gap between fetched and max id
	messageId1 := messageStore.EXPECT().DoInTx(gomock.Any(), gomock.Any()).
		Do(func(partition string, callback func(maxMessageId uint64) error) {
			callback(uint64(3))
		}).Return(errUnreadMsgsAvailable)
	messageId1.After(fetch_first1)

	// fetch again, starting at 3, because, there is still a gap
	fetch_first2 := messageStore.EXPECT().Fetch(gomock.Any()).Do(func(r store.FetchRequest) {
		go func() {
			a.Equal("foo", r.Partition)
			a.Equal(1, r.Direction)
			a.Equal(uint64(3), r.StartId)
			a.Equal(int(math.MaxInt32), r.Count)

			r.StartCallback <- 1
			r.MessageC <- store.MessageAndId{Id: uint64(3), Message: []byte("fetch_first2-a")}
			close(r.MessageC)
		}()
	})
	fetch_first2.After(messageId1)

	// the gap is closed
	messageId2 := messageStore.EXPECT().DoInTx(gomock.Any(), gomock.Any()).
		Do(func(partition string, callback func(maxMessageId uint64) error) {
			callback(uint64(3))
		})
	messageId2.After(fetch_first2)

	// subscribe
	subscribe := routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(r *Route) {
		a.Equal(r.Path, guble.Path("/foo"))
		r.C <- MsgAndRoute{Message: &guble.Message{Id: uint64(4), Body: []byte("router-a"), PublishingTime: 1405544146}, Route: r}
		r.C <- MsgAndRoute{Message: &guble.Message{Id: uint64(5), Body: []byte("router-b"), PublishingTime: 1405544146}, Route: r}
		close(r.C) // emulate router close
	})
	subscribe.After(messageId2)

	// router closed, so we fetch again, starting at 6 (after meesages from subscribe)
	fetch_after := messageStore.EXPECT().Fetch(gomock.Any()).Do(func(r store.FetchRequest) {
		go func() {
			a.Equal(uint64(6), r.StartId)
			a.Equal(int(math.MaxInt32), r.Count)

			r.StartCallback <- 1
			r.MessageC <- store.MessageAndId{Id: uint64(6), Message: []byte("fetch_after-a")}
			close(r.MessageC)
		}()
	})
	fetch_after.After(subscribe)

	// no gap
	messageId3 := messageStore.EXPECT().DoInTx(gomock.Any(), gomock.Any()).
		Do(func(partition string, callback func(maxMessageId uint64) error) {
			callback(uint64(6))
		})

	messageId3.After(fetch_after)

	// subscribe and don't send messages,
	// so the client has to wait until we stop
	subscribe2 := routerMock.EXPECT().Subscribe(gomock.Any())
	subscribe2.After(messageId3)

	subscriptionLoopDone := make(chan bool)
	go func() {
		rec.subscriptionLoop()
		subscriptionLoopDone <- true
	}()

	expectMessages(a, msgChannel,
		"#"+guble.SUCCESS_FETCH_START+" /foo 2",
		"fetch_first1-a",
		"fetch_first1-b",
		"#"+guble.SUCCESS_FETCH_END+" /foo",
		"#"+guble.SUCCESS_FETCH_START+" /foo 1",
		"fetch_first2-a",
		"#"+guble.SUCCESS_FETCH_END+" /foo",
		"#"+guble.SUCCESS_SUBSCRIBED_TO+" /foo",
		",4,,,,1405544146\n\nrouter-a",
		",5,,,,1405544146\n\nrouter-b",
		"#"+guble.SUCCESS_FETCH_START+" /foo 1",
		"fetch_after-a",
		"#"+guble.SUCCESS_FETCH_END+" /foo",
		"#"+guble.SUCCESS_SUBSCRIBED_TO+" /foo",
	)

	time.Sleep(time.Millisecond)
	routerMock.EXPECT().Unsubscribe(gomock.Any())
	rec.Stop()

	expectMessages(a, msgChannel,
		"#"+guble.SUCCESS_CANCELED+" /foo",
	)

	expectDone(a, subscriptionLoopDone)
}

func Test_Receiver_Fetch_Returns_Correct_Messages(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	rec, msgChannel, _, messageStore, err := aMockedReceiver("/foo 0 2")
	a.NoError(err)

	messages := []string{"The answer ", "is 42"}
	done := make(chan bool)
	messageStore.EXPECT().Fetch(gomock.Any()).Do(func(r store.FetchRequest) {
		go func() {
			r.StartCallback <- len(messages)
			for i, m := range messages {
				r.MessageC <- store.MessageAndId{Id: uint64(i + 1), Message: []byte(m)}
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
	expectDone(a, done)

	expectMessages(a, msgChannel, "#"+guble.SUCCESS_FETCH_START+" /foo 2")
	expectMessages(a, msgChannel, messages...)
	expectMessages(a, msgChannel, "#"+guble.SUCCESS_FETCH_END+" /foo")

	expectDone(a, fetchHasTerminated)
	ctrl.Finish()
}

func Test_Receiver_Fetch_Produces_Correct_Fetch_Requests(t *testing.T) {
	a := assert.New(t)

	testcases := []struct {
		desc   string
		arg    string
		maxId  int
		expect store.FetchRequest
	}{
		{desc: "simple forward fetch",
			arg:    "/foo 0 20",
			maxId:  -1,
			expect: store.FetchRequest{Partition: "foo", Direction: 1, StartId: uint64(0), Count: 20},
		},
		{desc: "forward fetch without bounds",
			arg:    "/foo 0",
			maxId:  -1,
			expect: store.FetchRequest{Partition: "foo", Direction: 1, StartId: uint64(0), Count: math.MaxInt32},
		},
		{desc: "backward fetch to top",
			arg:    "/foo -20",
			maxId:  42,
			expect: store.FetchRequest{Partition: "foo", Direction: 1, StartId: uint64(23), Count: 20},
		},
		{desc: "backward fetch with count",
			arg:    "/foo -1 10",
			maxId:  42,
			expect: store.FetchRequest{Partition: "foo", Direction: 1, StartId: uint64(42), Count: 10},
		},
	}

	for _, test := range testcases {
		ctrl = gomock.NewController(t)

		rec, _, _, messageStore, err := aMockedReceiver(test.arg)
		a.NoError(err, test.desc)

		if test.maxId != -1 {
			messageStore.EXPECT().MaxMessageId(test.expect.Partition).
				Return(uint64(test.maxId), nil)
		}

		done := make(chan bool)
		messageStore.EXPECT().Fetch(gomock.Any()).Do(func(r store.FetchRequest) {
			a.Equal(test.expect.Partition, r.Partition, test.desc)
			a.Equal(test.expect.Direction, r.Direction, test.desc)
			a.Equal(test.expect.StartId, r.StartId, test.desc)
			a.Equal(test.expect.Count, r.Count, test.desc)
			done <- true
		})

		go rec.fetchOnlyLoop()
		expectDone(a, done)
		ctrl.Finish()
		rec.Stop()
	}
}

func Test_Receiver_Fetch_Sends_error_on_failure(t *testing.T) {
	a := assert.New(t)

	for _, arg := range []string{
		"/foo 0 2", // fetch only
		"/foo 0",   // fetch and subscribe
	} {
		ctrl = gomock.NewController(t)

		rec, msgChannel, _, messageStore, err := aMockedReceiver(arg)
		a.NoError(err)

		messageStore.EXPECT().Fetch(gomock.Any()).Do(func(r store.FetchRequest) {
			go func() {
				r.ErrorCallback <- errors.New("expected test error")
			}()
		})

		rec.Start()
		expectMessages(a, msgChannel, "!error-server-internal expected test error")
		ctrl.Finish()
	}
}

func Test_Receiver_Fetch_Sends_error_on_failure_in_MaxMessageId(t *testing.T) {
	defer initCtrl(t)()
	a := assert.New(t)

	rec, msgChannel, _, messageStore, err := aMockedReceiver("/foo -2 2")
	a.NoError(err)

	messageStore.EXPECT().MaxMessageId("foo").
		Return(uint64(0), errors.New("expected test error"))

	rec.Start()

	expectMessages(a, msgChannel, "!error-server-internal expected test error")
}

//rec, sendChannel, pubSubSource, messageStore, err := aMockedReceiver("+")
func aMockedReceiver(arg string) (*Receiver, chan []byte, *MockPubSubSource, *MockMessageStore, error) {
	pubSubSource := NewMockPubSubSource(ctrl)
	messageStore := NewMockMessageStore(ctrl)
	sendChannel := make(chan []byte)
	cmd := &guble.Cmd{
		Name: guble.CmdReceive,
		Arg:  arg,
	}
	rec, err := NewReceiverFromCmd("any-appId", cmd, sendChannel, pubSubSource, messageStore, "userId")
	return rec, sendChannel, pubSubSource, messageStore, err
}

func expectMessages(a *assert.Assertions, msgChannel chan []byte, message ...string) {
	for _, m := range message {
		select {
		case msg := <-msgChannel:
			a.Equal(m, string(msg))
		case <-time.After(time.Millisecond * 100):
			a.Fail("timeout: " + m)
		}
	}
}
