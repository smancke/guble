package metrics

import (
	"github.com/smancke/guble/gubled/config"
	"github.com/stretchr/testify/assert"

	log "github.com/Sirupsen/logrus"

	"bytes"
	"expvar"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewInt_Disabled(t *testing.T) {
	_, ok := NewInt("a_name").(expvar.Var)
	assert.False(t, ok)
}

func TestNewInt_Enabled(t *testing.T) {
	*config.Metrics.Enabled = true
	_, ok := NewInt("a_name").(expvar.Var)
	assert.True(t, ok)
	*config.Metrics.Enabled = false
}

func TestHttpHandler_MetricsNotEnabled(t *testing.T) {
	a := assert.New(t)
	req, _ := http.NewRequest("GET", "", nil)
	w := httptest.NewRecorder()
	HttpHandler(w, req)
	a.Equal(http.StatusOK, w.Code)
	b, err := ioutil.ReadAll(w.Body)
	a.NoError(err)
	a.True(len(b) > 0)
	log.Debugf("%s", b)
}

func TestLogOnDebugLevel_DebugAndEnabled(t *testing.T) {
	a := assert.New(t)
	bufferDebug := bytes.NewBuffer([]byte{})
	log.SetOutput(bufferDebug)

	log.SetLevel(log.DebugLevel)

	LogOnDebugLevel()

	logContent, err := ioutil.ReadAll(bufferDebug)
	a.NoError(err)

	a.Contains(string(logContent), "metrics: not enabled")
}

func TestLogOnDebugLevel_DebugAndDisabled(t *testing.T) {
	a := assert.New(t)
	bufferDebug := bytes.NewBuffer([]byte{})
	log.SetOutput(bufferDebug)

	log.SetLevel(log.DebugLevel)

	*config.Metrics.Enabled = true
	LogOnDebugLevel()
	*config.Metrics.Enabled = false

	logContent, err := ioutil.ReadAll(bufferDebug)
	a.NoError(err)

	a.Contains(string(logContent), "cmdline")
	a.Contains(string(logContent), "memstats")
}

func TestLogOnDebugLevel_Info(t *testing.T) {
	a := assert.New(t)
	bufferInfo := bytes.NewBuffer([]byte{})
	log.SetOutput(bufferInfo)

	log.SetLevel(log.InfoLevel)

	logContent, err := ioutil.ReadAll(bufferInfo)
	a.NoError(err)

	*config.Metrics.Enabled = true
	LogOnDebugLevel()
	*config.Metrics.Enabled = false

	a.True(len(logContent) == 0)
}
