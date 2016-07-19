package kv

import "strings"

type PostgresConfig struct {
	ConnParams   map[string]string
	MaxIdleConns int
	MaxOpenConns int
}

func (pc PostgresConfig) connectionString() string {
	var params []string
	for key, value := range pc.ConnParams {
		params = append(params, key+"="+value)
	}
	return strings.Join(params, " ")
}
