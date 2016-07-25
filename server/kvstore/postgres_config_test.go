package kvstore

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPostgresConfig_String(t *testing.T) {
	a := assert.New(t)
	pc0 := PostgresConfig{map[string]string{}, 1, 1}
	a.Equal(pc0.connectionString(), "")

	pc1 := PostgresConfig{map[string]string{"key": "value"}, 1, 1}
	a.Equal(pc1.connectionString(), "key=value")

	pc2 := PostgresConfig{map[string]string{"key": "value", "password": "secret"}, 1, 1}
	s := pc2.connectionString()
	a.True(s == "key=value password=secret" || s == "password=secret key=value")
}
