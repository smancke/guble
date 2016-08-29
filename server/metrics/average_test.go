package metrics

import (
	"github.com/stretchr/testify/assert"

	"testing"
)

func TestAverage_String(t *testing.T) {
	assert.Equal(t, "x", newAverage(0, 0, 0, "x").String())
	assert.Equal(t, "x", newAverage(0, 1, 0, "x").String())
	assert.Equal(t, "0", newAverage(0, 1, 1, "").String())
	assert.Equal(t, "1", newAverage(1, 1, 1, "").String())
	assert.Equal(t, "2", newAverage(40, 2, 10, "").String())
}
