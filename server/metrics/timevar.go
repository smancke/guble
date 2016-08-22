package metrics

import (
	"fmt"
	"time"
)

type Time struct {
	timeValue time.Time
}

func NewTime(timeValue time.Time) Time {
	return Time{timeValue: timeValue}
}

func (t Time) String() string {
	return fmt.Sprintf("\"%v\"", t.timeValue)
}
