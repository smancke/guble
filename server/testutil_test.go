package server

import (
	"github.com/smancke/guble/guble"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var ctrl *gomock.Controller
var testBytes = []byte("test")

func init() {
	// disable error output while testing
	// because also negative tests are tested
	guble.LogLevel = guble.LEVEL_ERR + 1
}

func initCtrl(t *testing.T) func() {
	ctrl = gomock.NewController(t)
	return func() { ctrl.Finish() }
}

func enableDebugForMethod() func() {
	reset := guble.LogLevel
	guble.LogLevel = guble.LEVEL_DEBUG
	return func() { guble.LogLevel = reset }
}

func expectDone(a *assert.Assertions, doneChannel chan bool) {
	select {
	case <-doneChannel:
		return
	case <-time.After(time.Second):
		a.Fail("timeout")
	}
}
