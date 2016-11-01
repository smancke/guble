package service

import (
	"net/http"
	"sort"
)

// Startable interface for modules which provide a start mechanism
type Startable interface {
	Start() error
}

// Stopable interface for modules which provide a stop mechanism
type Stopable interface {
	Stop() error
}

// Endpoint adds a HTTP handler for the `GetPrefix()` to the webserver
type Endpoint interface {
	http.Handler
	GetPrefix() string
}

type module struct {
	iface      interface{}
	startLevel int
	stopLevel  int
}

type by func(m1, m2 *module) bool

type moduleSorter struct {
	modules []module
	by      func(m1, m2 *module) bool
}

func (criteria by) sort(modules []module) {
	ms := &moduleSorter{
		modules: modules,
		by:      criteria,
	}
	sort.Sort(ms)
}

// functions implementing the sort.Interface

func (s *moduleSorter) Len() int           { return len(s.modules) }
func (s *moduleSorter) Swap(i, j int)      { s.modules[i], s.modules[j] = s.modules[j], s.modules[i] }
func (s *moduleSorter) Less(i, j int) bool { return s.by(&s.modules[i], &s.modules[j]) }

var ascendingStartOrder = func(m1, m2 *module) bool {
	return m1.startLevel < m2.startLevel
}

var ascendingStopOrder = func(m1, m2 *module) bool {
	return m1.stopLevel < m2.stopLevel
}
