// +build !disablemetrics

package metrics

import (
	"expvar"
)

// NewInt returns an expvar Int, depending on the absence of build tag declared at the beginning of this file
func NewInt(name string) IntVar {
	return expvar.NewInt(name)
}
