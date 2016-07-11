package server

import (
	"net/http"
	"sort"
)

// startable interface for modules which provide a start mechanism
type startable interface {
	Start() error
}

// stopable interface for modules which provide a stop mechanism
type stopable interface {
	Stop() error
}

// endpoint adds a HTTP handler for the `GetPrefix()` to the webserver
type endpoint interface {
	http.Handler
	GetPrefix() string
}

type module struct {
	iface      interface{}
	startOrder int // starting in ascending order (e.g. first negative, then positive numbers)
	stopOrder  int // stopping in ascending order
}

type by func(m1, m2 *module) bool

type moduleSorter struct {
	modules []module
	by      func(m1, m2 *module) bool
}

func (b by) sort(modules []module) {
	ps := &moduleSorter{
		modules: modules,
		by:      b,
	}
	sort.Sort(ps)
}

func (s *moduleSorter) Len() int { return len(s.modules) }

func (s *moduleSorter) Swap(i, j int) { s.modules[i], s.modules[j] = s.modules[j], s.modules[i] }

func (s *moduleSorter) Less(i, j int) bool { return s.by(&s.modules[i], &s.modules[j]) }

var byStartOrder = func(m1, m2 *module) bool {
	return m1.startOrder < m2.startOrder
}

var byStopOrder = func(m1, m2 *module) bool {
	return m1.stopOrder < m2.stopOrder
}
