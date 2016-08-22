package metrics

import "fmt"

type average struct {
	total            int64
	cases            int64
	defaultJSONValue string
}

func newAverage(total int64, cases int64, defaultAverageJSONValue string) average {
	return average{
		total:            total,
		cases:            cases,
		defaultJSONValue: defaultAverageJSONValue,
	}
}

func (a average) String() string {
	if a.cases <= 0 {
		return a.defaultJSONValue
	}
	return fmt.Sprintf("%v", a.total/a.cases)
}
