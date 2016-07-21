package metrics

import (
	"github.com/stretchr/testify/assert"

	"expvar"
	"testing"
)

func TestNewInt(t *testing.T) {
	_, ok := NewInt("a_name").(expvar.Var)
	assert.True(t, ok)
}
