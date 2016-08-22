// +build !disablemetrics

package metrics

import (
	"expvar"
	"time"
)

// NewInt returns an expvar Int, depending on the absence of build tag declared at the beginning of this file
func NewInt(name string) IntVar {
	return expvar.NewInt(name)
}

func Every(d time.Duration, f func(*expvar.Map, time.Time), m *expvar.Map) {
	for x := range time.Tick(d) {
		f(m, x)
	}
}
