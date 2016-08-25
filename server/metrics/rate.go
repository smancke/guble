package metrics

import (
	"fmt"
	"time"
)

type rate struct {
	value      int64
	multiplier float64
}

func newRate(value int64, timeframe, scale time.Duration) rate {
	return rate{
		value:      value,
		multiplier: float64(scale.Nanoseconds()) / float64(timeframe.Nanoseconds()),
	}
}

func (r rate) String() string {
	if r.value <= 0 {
		return "0.0"
	}
	return fmt.Sprintf("%v", float64(r.value)*r.multiplier)
}
