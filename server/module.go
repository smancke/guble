package server

import (
	"net/http"
)

// Startable interface for modules which provide a start mechanism
type startable interface {
	Start() error
}

// Stopable interface for modules which provide a stop mechanism
type stopable interface {
	Stop() error
}

// Endpoint adds a HTTP handler for the `GetPrefix()` to the webserver
type endpoint interface {
	http.Handler
	GetPrefix() string
}

type module struct {
	iface      interface{}
	startOrder int // starting in ascending order (e.g. first negative, then positive numbers)
	stopOrder  int // stopping in ascending order
}

type startSorter []module

func (slice startSorter) Len() int           { return len(slice) }
func (slice startSorter) Swap(i, j int)      { slice[i], slice[j] = slice[j], slice[i] }
func (slice startSorter) Less(i, j int) bool { return slice[i].startOrder < slice[j].startOrder }

type stopSorter []module

func (slice stopSorter) Len() int           { return len(slice) }
func (slice stopSorter) Swap(i, j int)      { slice[i], slice[j] = slice[j], slice[i] }
func (slice stopSorter) Less(i, j int) bool { return slice[i].stopOrder < slice[j].stopOrder }
