// +build disablemetrics

package metrics

type dummyInt struct{}

// Dummy functions on dummyInt
func (v *dummyInt) Add(delta int64) {}

func (v *dummyInt) Set(value int64) {}

// NewInt returns a dummyInt, depending on the build tag declared at the beginning of this file.
func NewInt(name string) IntVar {
	return &dummyInt{}
}

func Every(d time.Duration, f func(*expvar.Map, time.Time), m *expvar.Map) {
}
