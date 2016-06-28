package store

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPostgresConfig_String(t *testing.T) {
	a := assert.New(t)
	pc1 := PostgresConfig{"key": "value"}
	a.Equal(pc1.String(), "key=value")
	pc2 := PostgresConfig{"key": "value", "password": "secret"}
	s := pc2.String()
	a.True(s == "key=value password=secret" || s == "password=secret key=value")
}
