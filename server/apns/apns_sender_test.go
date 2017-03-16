package apns

import (
	"errors"
	"github.com/golang/mock/gomock"
	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/router"
	"github.com/cosminrentea/gobbler/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewSender_ErrorBytes(t *testing.T) {
	a := assert.New(t)

	//given
	emptyBytes := []byte("")
	emptyPassword := ""
	cfg := Config{
		CertificateBytes:    &emptyBytes,
		CertificatePassword: &emptyPassword,
	}

	//when
	pusher, err := NewSender(cfg)

	// then
	a.Error(err)
	a.Nil(pusher)
}

func TestNewSender_ErrorFile(t *testing.T) {
	a := assert.New(t)

	//given
	wrongFilename := "."
	emptyPassword := ""
	cfg := Config{
		CertificateFileName: &wrongFilename,
		CertificatePassword: &emptyPassword,
	}

	//when
	pusher, err := NewSender(cfg)

	// then
	a.Error(err)
	a.Nil(pusher)
}

func TestSender_Send(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	// given
	routeParams := make(map[string]string)
	routeParams["device_id"] = "1234"
	routeConfig := router.RouteConfig{
		Path:        protocol.Path("path"),
		RouteParams: routeParams,
	}
	route := router.NewRoute(routeConfig)

	msg := &protocol.Message{
		Body: []byte("{}"),
	}

	mSubscriber := NewMockSubscriber(testutil.MockCtrl)
	mSubscriber.EXPECT().Route().Return(route).AnyTimes()

	mRequest := NewMockRequest(testutil.MockCtrl)
	mRequest.EXPECT().Subscriber().Return(mSubscriber).AnyTimes()
	mRequest.EXPECT().Message().Return(msg).AnyTimes()

	mPusher := NewMockPusher(testutil.MockCtrl)
	mPusher.EXPECT().Push(gomock.Any()).Return(nil, nil)

	// and
	s, err := NewSenderUsingPusher(mPusher, "com.myapp")
	a.NoError(err)

	// when
	rsp, err := s.Send(mRequest)

	// then
	a.NoError(err)
	a.Nil(rsp)
}

func TestSender_Retry(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	// given
	routeParams := make(map[string]string)
	routeParams["device_id"] = "1234"
	routeConfig := router.RouteConfig{
		Path:        protocol.Path("path"),
		RouteParams: routeParams,
	}
	route := router.NewRoute(routeConfig)

	msg := &protocol.Message{
		Body: []byte("{}"),
	}

	mSubscriber := NewMockSubscriber(testutil.MockCtrl)
	mSubscriber.EXPECT().Route().Return(route).AnyTimes()

	mRequest := NewMockRequest(testutil.MockCtrl)
	mRequest.EXPECT().Subscriber().Return(mSubscriber).AnyTimes()
	mRequest.EXPECT().Message().Return(msg).AnyTimes()

	mPusher := NewMockPusher(testutil.MockCtrl)

	mPusher.EXPECT().Push(gomock.Any()).Return(nil, errMockTimeout)
	mPusher.EXPECT().Push(gomock.Any()).Return(nil, nil)

	// and
	s, err := NewSenderUsingPusher(mPusher, "com.myapp")
	a.NoError(err)

	// when
	rsp, err := s.Send(mRequest)

	// then
	a.NoError(err)
	a.Nil(rsp)

}

type resultpair struct {
	result interface{}
	err    error
}

func Test_Retriable(t *testing.T) {
	a := assert.New(t)

	testCases := []struct {
		name                string
		maxTries            int
		results             []resultpair
		expectedResult      string
		expectedError       error
		expectedMethodCalls int
	}{
		{"No errors", 3, []resultpair{{result: "0"}}, "0", nil, 1},
		{"Retry once", 3, []resultpair{{err: errMockTimeout}, {result: "1"}}, "1", nil, 2},
		{"Retry twice", 3, []resultpair{{err: errMockTimeout}, {err: errMockTimeout}, {result: "2"}}, "2", nil, 3},
		{"Retry only twice", 3, []resultpair{{err: errMockTimeout}, {err: errMockTimeout}, {err: errMockTimeout, result: ""}, {result: "3"}}, "", ErrRetryFailed, 3},
		{"Do not retry", 3, []resultpair{{err: errMockOther, result: ""}, {result: "1"}}, "", errMockOther, 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// given
			var counter int
			method := func() (interface{}, error) {
				elem := tc.results[counter]
				counter++
				return elem.result, elem.err
			}

			withRetry := &retryable{
				maxTries: tc.maxTries,
			}

			// when
			result, err := withRetry.execute(method)

			// then
			a.EqualValues(tc.expectedResult, result)
			a.EqualValues(tc.expectedError, err)
			a.EqualValues(tc.expectedMethodCalls, counter)
		})
	}
}

// see - net.Error
type mockTimeout struct{}

func (e *mockTimeout) Error() string   { return "mock i/o timeout" }
func (e *mockTimeout) Timeout() bool   { return true }
func (e *mockTimeout) Temporary() bool { return true }

var errMockTimeout error = &mockTimeout{}
var errMockOther error = errors.New("mock not retriable")
