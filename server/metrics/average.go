package metrics

import "fmt"

type Average struct {
	total                   int64
	cases                   int64
	defaultAverageJSONValue string
}

func NewAverage(total int64, cases int64, defaultAverageJSONValue string) Average {
	return Average{
		total: total,
		cases: cases,
		defaultAverageJSONValue: defaultAverageJSONValue,
	}
}

func (a Average) String() string {
	if a.cases <= 0 {
		return a.defaultAverageJSONValue
	}
	return fmt.Sprintf("%v", a.total/a.cases)
}
