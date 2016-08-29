// +build !disablemetrics

package metrics

import (
	"expvar"
	"time"
)

// NewInt returns an expvar Int, depending on the absence of build tag declared at the beginning of this file
func NewInt(name string) Int {
	return expvar.NewInt(name)
}

func NewMap(name string) Map {
	return expvar.NewMap(name)
}

func Every(d time.Duration, f func(Map, time.Duration, time.Time), m Map) {
	for t := range time.Tick(d) {
		f(m, d, t)
	}
}
