package metrics

import (
	"github.com/stretchr/testify/assert"

	log "github.com/Sirupsen/logrus"

	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

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

func TestLogOnDebugLevel_Debug(t *testing.T) {
	a := assert.New(t)
	bufferDebug := bytes.NewBuffer([]byte{})
	log.SetOutput(bufferDebug)

	log.SetLevel(log.DebugLevel)

	LogOnDebugLevel()

	logContent, err := ioutil.ReadAll(bufferDebug)
	log.Debugf("%s", logContent)
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

	LogOnDebugLevel()

	a.True(len(logContent) == 0)
}
