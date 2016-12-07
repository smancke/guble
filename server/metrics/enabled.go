// +build !disablemetrics

package metrics

import (
	"context"
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

func RegisterInterval(ctx context.Context, m Map, td time.Duration, reset func(Map, time.Time), processAndReset func(Map, time.Duration, time.Time)) {
	reset(m, time.Now())
	go func(m Map, td time.Duration, processAndReset func(Map, time.Duration, time.Time)) {
		for {
			select {
			case t := <-time.Tick(td):
				processAndReset(m, td, t)
			case <-ctx.Done():
				return
			}
		}
	}(m, td, processAndReset)
}
