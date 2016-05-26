package testutil

import (
	"github.com/golang/mock/gomock"
	"github.com/smancke/guble/protocol"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// MockCtrl is a gomock.Controller to use globally
var MockCtrl *gomock.Controller

func init() {
	// disable error output while testing
	// because also negative tests are tested
	protocol.LogLevel = protocol.LEVEL_ERR
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

// EnableDebugForMethod enables debug output throught the current test
// Usage:
//		test_util.EnableDebugForMethod()()
func EnableDebugForMethod() func() {
	reset := protocol.LogLevel
	protocol.LogLevel = protocol.LEVEL_DEBUG
	return func() { protocol.LogLevel = reset }
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
