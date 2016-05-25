package protocol

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"testing"
)

func Test_log_functions(t *testing.T) {
	a := assert.New(t)

	w := bytes.NewBuffer([]byte{})
	log.SetOutput(w)
	defer log.SetOutput(os.Stderr)

	LogLevel = LEVEL_DEBUG
	a.True(DebugEnabled())
	a.True(InfoEnabled())
	a.True(WarnEnabled())

	Debug("debug")
	a.Contains(w.String(), "DEBUG: debug")

	w.Reset()
	Info("info")
	a.Contains(w.String(), "INFO: info")

	w.Reset()
	Warn("warn")
	a.Contains(w.String(), "WARN: warn")

	w.Reset()
	Err("err")
	a.Contains(w.String(), "ERROR")
	a.Contains(w.String(), "protocol.Test_log_functions")
	a.Contains(w.String(), "err")

	w.Reset()
	ErrWithoutTrace("err")
	a.Contains(w.String(), "ERROR")
	a.Contains(w.String(), "err")
}

func Test_log_functions_only_error_enabled(t *testing.T) {
	a := assert.New(t)

	w := bytes.NewBuffer([]byte{})
	log.SetOutput(w)
	defer log.SetOutput(os.Stderr)

	LogLevel = LEVEL_ERR
	a.False(DebugEnabled())
	a.False(InfoEnabled())
	a.False(WarnEnabled())

	Debug("debug")
	Info("info")
	Warn("warn")
	a.Equal("", w.String())

	Err("err")
	a.Contains(w.String(), "ERROR")
	a.Contains(w.String(), "protocol.Test_log_functions")
	a.Contains(w.String(), "err")
}

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
