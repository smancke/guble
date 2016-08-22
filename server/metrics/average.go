package metrics

import "fmt"

type Average struct {
	total            int64
	cases            int64
	defaultJSONValue string
}

func NewAverage(total int64, cases int64, defaultAverageJSONValue string) Average {
	return Average{
		total:            total,
		cases:            cases,
		defaultJSONValue: defaultAverageJSONValue,
	}
}

func (a Average) String() string {
	if a.cases <= 0 {
		return a.defaultJSONValue
	}
	return fmt.Sprintf("%v", a.total/a.cases)
}
