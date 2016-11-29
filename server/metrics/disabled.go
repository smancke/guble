// +build disablemetrics

package metrics

import (
	"expvar"
	"time"
)

type dummyInt struct{}

// Dummy functions on dummyInt
func (v *dummyInt) Add(delta int64) {}
func (v *dummyInt) Set(value int64) {}

// NewInt returns a dummyInt, depending on the build tag declared at the beginning of this file.
func NewInt(name string) Int {
	return &dummyInt{}
}

type dummyMap struct{}

// Dummy functions on dummyMap
func (v *dummyMap) Init() *expvar.Map             { return nil }
func (v *dummyMap) Get(key string) expvar.Var     { return nil }
func (v *dummyMap) Set(key string, av expvar.Var) {}
func (v *dummyMap) Add(key string, delta int64)   {}

// NewMap returns a dummyMap, depending on the build tag declared at the beginning of this file.
func NewMap(name string) Map {
	return &dummyMap{}
}

func RegisterInterval(m Map, td time.Duration, reset func(Map, time.Time), processAndReset func(Map, time.Duration, time.Time)) {
}
