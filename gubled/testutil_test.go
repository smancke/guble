package gubled

import (
	"github.com/smancke/guble/guble"

	"github.com/golang/mock/gomock"
	"testing"
)

var ctrl *gomock.Controller
var testBytes = []byte("test")

func init() {
	// disable error output while testing
	// because also negative tests are tested
	guble.LogLevel = guble.LEVEL_WARN
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
