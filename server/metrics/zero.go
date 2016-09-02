package metrics

type zeroVar struct {
}

func (z zeroVar) String() string {
	return "0"
}

var zeroValue zeroVar
