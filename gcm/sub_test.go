package gcm

import (
	"github.com/alexjlockwood/gcm"
	"github.com/golang/mock/gomock"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
	"strconv"
	"strings"
	"testing"
	"time"
)

var fetchMessage = `/foo/bar,42,user01,phone01,id123,1420110000
{"Content-Type": "text/plain", "Correlation-Id": "7sdks723ksgqn"}
Hello World`

func TestSub_Fetch(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	gcm, routerMock := testSimpleGCM(t)

	route := server.NewRoute("/foo/bar", "phone01", "user01", subBufferSize)
	sub := newSub(gcm, route, 2)

	// simulate the fetch
	routerMock.EXPECT().Fetch(gomock.Any()).Do(func(req store.FetchRequest) {
		go func() {
			// send 2 messages from the store
			req.StartC <- 2
			var id uint64 = 3
			for i := 0; i < 2; i++ {
				req.MessageC <- store.MessageAndID{
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
		pm.resultC <- dummyGCMResponse()

		// pipe message
		pm = <-gcm.pipelineC
		a.Equal(uint64(4), pm.message.ID)
		pm.resultC <- dummyGCMResponse()

		close(done)
	}()

	// start subscription fetching
	go sub.fetch()

	select {
	case <-done:
		// all good
	case <-time.After(30 * time.Millisecond):
		// taking too long, fail the test
		a.Fail("Fetching messages and piping them took too long.")
	}
}

func dummyGCMResponse() *gcm.Response {
	return &gcm.Response{
		Results: []gcm.Result{{Error: ""}},
	}
}

// Test that if a route is closed, but no explicit shutdown the subscription will
// try to refetch messages from store and then resubscribe
func TestSub_Restart(t *testing.T) {

}
