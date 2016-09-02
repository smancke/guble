package gcm

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Bogh/gcm"
	"github.com/golang/mock/gomock"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/router"
	"github.com/smancke/guble/server/store"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
)

var (
	fetchMessage = `/foo/bar,42,user01,phone01,id123,1420110000,1
{"Content-Type": "text/plain", "Correlation-Id": "7sdks723ksgqn"}
Hello World`
	dummyGCMResponse = &gcm.Response{
		Results: []gcm.Result{{Error: ""}},
	}
	errorGCMNotRegisteredResponse = &gcm.Response{
		Results: []gcm.Result{{Error: "NotRegistered"}},
	}
)

func TestSub_Fetch(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	gcm, routerMock, _ := testSimpleGCM(t, false)

	route := router.NewRoute(router.RouteConfig{
		RouteParams: router.RouteParams{userIDKey: "user01", applicationIDKey: "phone01"},
		Path:        protocol.Path("/foo/bar"),
		ChannelSize: subBufferSize,
	})
	sub := newSubscription(gcm, route, 2)

	// simulate the fetch
	routerMock.EXPECT().Fetch(gomock.Any()).Do(func(req store.FetchRequest) {
		go func() {
			// send 2 messages from the store
			req.StartC <- 2
			var id uint64 = 3
			for i := 0; i < 2; i++ {
				req.MessageC <- store.FetchedMessage{
					ID:      id,
					Message: []byte(strings.Replace(fetchMessage, "42", strconv.FormatUint(id, 10), 1)),
				}
				id++
			}
			close(req.MessageC)
		}()
	})

	done := make(chan struct{})

	// read messages from gcm pipeline, must read 2 messages
	go func() {
		// pipe message
		pm := <-gcm.pipelineC
		a.Equal(uint64(3), pm.message.ID)
		// acknowledge the response
		pm.resultC <- dummyGCMResponse

		// pipe message
		pm = <-gcm.pipelineC
		a.Equal(uint64(4), pm.message.ID)
		pm.resultC <- dummyGCMResponse

		close(done)
	}()

	go func() {
		select {
		case <-done:
			// all good
		case <-time.After(30 * time.Millisecond):
			// taking too long, fail the test
			a.Fail("Fetching messages and piping them took too long.")
		}
	}()

	// start subscription fetching
	err := sub.fetch()
	a.NoError(err)

}

// Test that if a route is closed, but no explicit shutdown the subscription will
// try to refetch messages from store and then resubscribe
func TestSub_Restart(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	gcm, routerMock, storeMock := testSimpleGCM(t, true)

	route := router.NewRoute(router.RouteConfig{
		RouteParams: router.RouteParams{userIDKey: "user01", applicationIDKey: "phone01"},
		Path:        protocol.Path("/foo/bar"),
		ChannelSize: subBufferSize,
	})
	sub := newSubscription(gcm, route, 2)

	// start goroutine that will take the messages from the pipeline
	done := make(chan struct{})
	go func() {
		for {
			select {
			case pm := <-gcm.pipelineC:
				pm.resultC <- dummyGCMResponse
			case <-done:
				return
			}
		}
	}()

	routerMock.EXPECT().Subscribe(gomock.Eq(route))

	// expect again for a subscription
	routerMock.EXPECT().Subscribe(gomock.Any())
	storeMock.EXPECT().MaxMessageID("foo").Return(uint64(4), nil).AnyTimes()

	sub.start()

	time.Sleep(10 * time.Millisecond)

	// simulate the fetch
	routerMock.EXPECT().Fetch(gomock.Any()).Do(func(req store.FetchRequest) {
		go func() {
			// send 2 messages from the store
			req.StartC <- 2
			var id uint64 = 3
			for i := 0; i < 2; i++ {
				req.MessageC <- store.FetchedMessage{
					ID:      id,
					Message: []byte(strings.Replace(fetchMessage, "42", strconv.FormatUint(id, 10), 1)),
				}
				id++
			}
			close(req.MessageC)
		}()
	})
	route.Close()

	time.Sleep(20 * time.Millisecond)

	// subscription route shouldn't be equal anymore
	a.NotEqual(route, sub.route)
	a.Equal(uint64(4), sub.lastID)

	time.Sleep(10 * time.Millisecond)
	close(done)
}

func TestSubscription_JSONError(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	gcm, routerMock, _ := testSimpleGCM(t, true)
	routerMock.EXPECT().Subscribe(gomock.Any())

	sub, err := initSubscription(gcm, "/foo/bar", "user01", "gcm01", 0, true)
	a.NoError(err)
	a.Equal(1, len(gcm.subscriptions))

	// start goroutine that will take the messages from the pipeline
	done := make(chan struct{})
	go func() {
		for {
			select {
			case pm := <-gcm.pipelineC:
				pm.resultC <- errorGCMNotRegisteredResponse
			case <-done:
				return
			}
		}
	}()

	routerMock.EXPECT().Unsubscribe(gomock.Eq(sub.route))

	sub.route.Deliver(&protocol.Message{
		Path: protocol.Path("/foo/bar"),
		Body: []byte("test message"),
	})

	// subscriptions should be removed at this point
	time.Sleep(time.Second)
	a.Equal(0, len(gcm.subscriptions))

	close(done)
}
