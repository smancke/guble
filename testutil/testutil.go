package testutil

import (
	log "github.com/Sirupsen/logrus"
	"github.com/alexjlockwood/gcm"
	"github.com/docker/distribution/health"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"io/ioutil"
	"net/http"
	"os/exec"
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

// EnableDebugForMethod enables debug output through the current test
// Usage:
//		test_util.EnableDebugForMethod()()
func EnableDebugForMethod() func() {
	reset := log.GetLevel()
	log.SetLevel(log.DebugLevel)
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
	CorrectGcmResponseMessageJSON = `
{
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

	ErrorResponseMessageJSON = `
{
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

// RoundTripperFunc mocks a https round tripper in order to not send the test request to GCM.
type RoundTripperFunc func(req *http.Request) *http.Response

func (rt RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	log.WithFields(log.Fields{
		"module":   "testing",
		"url_path": req.URL.Path,
	}).Debug("Served request for")

	return rt(req), nil
}

func CreateGcmSender(rt RoundTripperFunc) *gcm.Sender {
	log.WithFields(log.Fields{
		"module": "testing",
	}).Debug("Create GCM sender")
	httpClient := &http.Client{Transport: rt}
	return &gcm.Sender{ApiKey: "124", Http: httpClient}
}

func CreateRoundTripperWithJsonResponse(httpStatusCode int, messageBodyAsJSON string, doneC chan bool) RoundTripperFunc {

	log.WithFields(log.Fields{
		"module": "testing",
	}).Debug("CreateRoundTripperWithJsonResponse")
	return RoundTripperFunc(func(req *http.Request) *http.Response {
		log.WithFields(log.Fields{
			"module": "testing",
		}).Debug("RoundTripperFunc")

		if doneC != nil {
			defer func() {
				close(doneC)
			}()
		}

		resp := &http.Response{
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     make(http.Header),
			Body:       ioutil.NopCloser(strings.NewReader(messageBodyAsJSON)),
			Request:    req,
			StatusCode: httpStatusCode,
		}
		resp.Header.Add("Content-Type", "application/json")
		return resp
	})
}

func SyncStorage() error {
	cmdSync := exec.Command("sync")
	return cmdSync.Run()
}
