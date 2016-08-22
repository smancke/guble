package metrics

import (
	"expvar"
	"strconv"
)

func SetCounter(m Map, key string, value expvar.Var) {
	if value != nil {
		m.Set(key, value)
	} else {
		m.Set(key, zeroValue)
	}
}

func SetAverage(m Map, key string, totalVar, casesVar expvar.Var, defaultValue string) {
	if totalVar != nil && casesVar != nil {
		total, _ := strconv.ParseInt(totalVar.String(), 10, 64)
		cases, _ := strconv.ParseInt(casesVar.String(), 10, 64)
		m.Set(key, newAverage(total, cases, defaultValue))
	} else {
		m.Set(key, zeroValue)
	}
}

func AddToMaps(key string, value int64, maps ...Map) {
	for _, m := range maps {
		m.Add(key, value)
	}
}
