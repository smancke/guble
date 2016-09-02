package metrics

import (
	"github.com/stretchr/testify/assert"

	"testing"
)

func TestRate_String(t *testing.T) {
	assert.Equal(t, "0", newRate(0, 0, 0).String())
	assert.Equal(t, "0", newRate(1, 0, 0).String())
	assert.Equal(t, "1", newRate(1, 1, 1).String())
	assert.Equal(t, "1.5", newRate(90, 60000, 1000).String())
	assert.Equal(t, "1.6666666666666667", newRate(100, 60000, 1000).String())
}
