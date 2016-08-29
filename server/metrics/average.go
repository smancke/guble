package metrics

import (
	"fmt"
)

type average struct {
	value string
}

func newAverage(total, cases, scale int64, defaultAverageJSONValue string) average {
	if cases <= 0 || scale <= 0 {
		return average{defaultAverageJSONValue}
	}
	return average{fmt.Sprintf("%v", float64(total)/float64(cases*scale))}
}

func (a average) String() string {
	return a.value
}
