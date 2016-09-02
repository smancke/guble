package metrics

// Int is an interface for some of the operations defined on expvar.Int
type Int interface {
	Add(int64)
	Set(int64)
}
