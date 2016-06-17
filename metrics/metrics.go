package metrics

import (
	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/gubled/config"

	"expvar"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Enabled is a global flag for enabling/disabling the collection of metrics.
// Metrics are enabled if a specific environment variable is defined with any value.
var Enabled = len(os.Getenv("GUBLE_METRICS")) > 0

// IntVar is an interface for the operations defined on expvar.Int
type IntVar interface {
	Add(int64)
	Set(int64)
}

type emptyInt struct{}

// Dummy functions on EmptyInt
func (v *emptyInt) Add(delta int64) {}

func (v *emptyInt) Set(value int64) {}

// NewInt returns an expvar.Int or a dummy emptyInt, depending on the Enabled flag
func NewInt(name string) IntVar {
	if *config.Metrics.Enabled {
		return expvar.NewInt(name)
	}
	return &emptyInt{}
}

// HttpHandler is a HTTP handler writing the current metrics to the http.ResponseWriter
func HttpHandler(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	writeMetrics(rw)
}

func writeMetrics(w io.Writer) {
	fmt.Fprintf(w, "{\n")
	first := true
	expvar.Do(func(kv expvar.KeyValue) {
		if !first {
			fmt.Fprintf(w, ",\n")
		}
		first = false
		fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
	})
	fmt.Fprintf(w, "\n}\n")
}

// LogOnDebugLevel logs all the current metrics, if logging is on Debug level.
func LogOnDebugLevel() {
	if !*config.Metrics.Enabled {
		log.Debug("metrics: not enabled")
		return
	}
	if log.GetLevel() == log.DebugLevel {
		fields := log.Fields{}
		expvar.Do(func(kv expvar.KeyValue) {
			fields[kv.Key] = kv.Value
		})
		log.WithFields(fields).Debug("metrics: current values")
	}
}
