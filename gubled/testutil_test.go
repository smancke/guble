package gubled

import (
	"github.com/smancke/guble/protocol"

	"github.com/golang/mock/gomock"
	"testing"
)

var ctrl *gomock.Controller
var testBytes = []byte("test")

func init() {
	// disable error output while testing
	// because also negative tests are tested
	protocol.LogLevel = protocol.LEVEL_ERR
}

func initCtrl(t *testing.T) func() {
	ctrl = gomock.NewController(t)
	return func() { ctrl.Finish() }
}

func enableDebugForMethod() func() {
	reset := protocol.LogLevel
	protocol.LogLevel = protocol.LEVEL_DEBUG
	return func() { protocol.LogLevel = reset }
}
