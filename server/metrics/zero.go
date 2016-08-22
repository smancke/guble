package metrics

type ZeroVar struct {
}

func (z ZeroVar) String() string {
	return "0"
}

var ZeroValue ZeroVar
