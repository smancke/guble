package gcm

import "fmt"

type Average struct {
	total int64
	cases int64
}

func newAverage(total int64, cases int64) *Average {
	return &Average{
		total: total,
		cases: cases,
	}
}

func (a *Average) String() string {
	if a.cases <= 0 {
		return "\"\""
	}
	return fmt.Sprintf("%v", a.total/a.cases)
}
