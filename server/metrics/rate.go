package metrics

import (
	"fmt"
	"time"
)

type rate struct {
	value string
}

func newRate(value int64, timeframe, scale time.Duration) rate {
	if value <= 0 || timeframe <= 0 || scale <= 0 {
		return rate{"0"}
	}
	return rate{fmt.Sprintf("%v", float64(value*scale.Nanoseconds())/float64(timeframe.Nanoseconds()))}
}

func (r rate) String() string {
	return r.value
}
