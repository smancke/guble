package metrics

import (
	"expvar"
)

// Map is an interface for some of the operations defined on expvar.Map
type Map interface {
	Init() *expvar.Map
	Get(key string) expvar.Var
	Set(key string, av expvar.Var)
	Add(key string, delta int64)
}
