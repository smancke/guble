package testutil

import (
	_ "net/http/pprof"

	"github.com/alexjlockwood/gcm"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/distribution/health"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"
)

// MockCtrl is a gomock.Controller to use globally
var MockCtrl *gomock.Controller

func init() {
	// disable error output while testing
	// because also negative tests are tested
	log.SetLevel(log.ErrorLevel)
}

// NewMockCtrl initializes the `MockCtrl` package var and returns a method to
// finish the controller when test is complete
// **Important**: Don't forget to call the returned method at the end of the test
// Usage:
// 		ctrl, finish := test_util.NewMockCtrl(t)
// 		defer finish()
func NewMockCtrl(t *testing.T) (*gomock.Controller, func()) {
	MockCtrl = gomock.NewController(t)
	return MockCtrl, func() { MockCtrl.Finish() }
}

// EnableDebugForMethod enables debug-level output through the current test
// Usage:
//		testutil.EnableDebugForMethod()()
func EnableDebugForMethod() func() {
	reset := log.GetLevel()
	log.SetLevel(log.DebugLevel)
	return func() { log.SetLevel(reset) }
}

// EnableDebugForMethod enables info-level output through the current test
// Usage:
//		testutil.EnableInfoForMethod()()
func EnableInfoForMethod() func() {
	reset := log.GetLevel()
	log.SetLevel(log.InfoLevel)
	return func() { log.SetLevel(reset) }
}

// ExpectDone waits to receive a value in the doneChannel for at least a second
// or fails the test.
func ExpectDone(a *assert.Assertions, doneChannel chan bool) {
	select {
	case <-doneChannel:
		return
	case <-time.After(time.Second):
		a.Fail("timeout in expectDone")
	}
}

// ExpectPanic expects a panic (and fails if this does not happen).
func ExpectPanic(t *testing.T) {
	if r := recover(); r == nil {
		assert.Fail(t, "Expecting a panic but unfortunately it did not happen")
	}
}

// ResetDefaultRegistryHealthCheck resets the existing registry containing health-checks
func ResetDefaultRegistryHealthCheck() {
	health.DefaultRegistry = health.NewRegistry()
}

const (
	SuccessGCMResponse = `{
	   "multicast_id":3,
	   "succes":1,
	   "failure":0,
	   "canonicals_ids":5,
	   "results":[
	      {
	         "message_id":"da",
	         "registration_id":"rId",
	         "error":""
	      }
	   ]
	}`

	ErrorGCMResponse = `{
	   "multicast_id":3,
	   "succes":0,
	   "failure":1,
	   "canonicals_ids":5,
	   "results":[
	      {
	         "message_id":"err",
	         "registration_id":"gcmCanonicalID",
	         "error":"InvalidRegistration"
	      }
	   ]
	}`
)

// RoundTripperFunc mocks/implements a http.RoundTripper in order to not send the test request to GCM.
type RoundTripperFunc func(req *http.Request) *http.Response

func (rt RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	log.WithFields(log.Fields{"module": "testing", "url_path": req.URL.Path}).Debug("Served request")
	return rt(req), nil
}

func CreateGcmSender(rt RoundTripperFunc) *gcm.Sender {
	log.WithFields(log.Fields{
		"module": "testing",
	}).Debug("Create GCM sender")
	httpClient := &http.Client{Transport: rt}
	return &gcm.Sender{ApiKey: "124", Http: httpClient}
}

func CreateRoundTripperWithJsonResponse(statusCode int, body string, doneC chan bool) RoundTripperFunc {
	log.WithFields(log.Fields{"module": "testing"}).Debug("CreateRoundTripperWithJSONResponse")

	return RoundTripperFunc(func(req *http.Request) *http.Response {
		log.WithFields(log.Fields{"module": "testing", "url": req.URL.String()}).Debug("RoundTripperFunc")

		if doneC != nil {
			defer func() {
				close(doneC)
			}()
		}
		resp := responseBuilder(statusCode, body)
		resp.Request = req
		return resp
	})
}

// CreateRoundTripperWithCountAndTimeout will mock the GCM API and will send each request as a count into a channel
func CreateRoundTripperWithCountAndTimeout(statusCode int, body string, countC chan bool, to time.Duration) RoundTripperFunc {
	log.WithFields(log.Fields{"module": "testing"}).Debug("CreateRoundTripperWithCount")
	return RoundTripperFunc(func(req *http.Request) *http.Response {
		defer func() {
			select {
			case countC <- true:
			case <-time.After(10 * time.Millisecond):
				return
			}
		}()
		log.WithFields(log.Fields{"module": "testing", "url": req.URL.String()}).Debug("RoundTripperWithCountAndTimeout")

		resp := responseBuilder(statusCode, body)
		resp.Request = req
		time.Sleep(to)
		return resp
	})
}

func responseBuilder(statusCode int, body string) *http.Response {
	headers := make(http.Header)
	headers.Add("Content-Type", "application/json")
	log.WithFields(log.Fields{"status": statusCode}).Debug("Building response")
	return &http.Response{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     headers,
		Body:       ioutil.NopCloser(strings.NewReader(body)),
		// Request:    req,
		StatusCode: statusCode,
	}
}

//SkipIfShort skips a test if the `-short` flag is given to `go test`
func SkipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
}

func PprofDebug() {
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()
}
