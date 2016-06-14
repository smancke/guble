package protocol

import (
	"bytes"
	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func Test_log_functions_panic_logger(t *testing.T) {
	a := assert.New(t)

	w := bytes.NewBuffer([]byte{})
	log.SetOutput(w)
	defer log.SetOutput(os.Stderr)

	raisePanic()

	a.Contains(w.String(), "PANIC")
	a.Contains(w.String(), "raisePanic")
	a.Contains(w.String(), "Don't panic!")
}

func raisePanic() {
	defer PanicLogger()
	panic("Don't panic!")
}
