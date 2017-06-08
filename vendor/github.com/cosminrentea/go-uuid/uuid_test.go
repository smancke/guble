package go_uuid

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_Lock_Unlock(t *testing.T) {
	a := assert.New(t)

	u, err := New()

	a.NoError(err)
	a.Equal(36, len(u))
}