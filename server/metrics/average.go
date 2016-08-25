package metrics

import "fmt"

type average struct {
	total            int64
	cases            int64
	scale            int64
	defaultJSONValue string
}

func newAverage(total, cases, scale int64, defaultAverageJSONValue string) average {
	return average{
		total:            total,
		cases:            cases,
		scale:            scale,
		defaultJSONValue: defaultAverageJSONValue,
	}
}

func (a average) String() string {
	if a.cases <= 0 {
		return a.defaultJSONValue
	}
	return fmt.Sprintf("%v", a.total/a.cases/a.scale)
}
