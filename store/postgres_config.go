package store

import "strings"

type PostgresConfig map[string]string

func (pc PostgresConfig) String() string {
	var params []string
	for key, value := range pc {
		params = append(params, key+"="+value)
	}
	return strings.Join(params, " ")
}
