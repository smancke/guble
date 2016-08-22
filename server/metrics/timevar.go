package metrics

import (
	"fmt"
	"time"
)

type TimeVar struct {
	timeValue time.Time
}

func NewTimeVar(timeValue time.Time) TimeVar {
	return TimeVar{timeValue: timeValue}
}

func (t TimeVar) String() string {
	return fmt.Sprintf("\"%v\"", t.timeValue)
}
