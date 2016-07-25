package kvstore

import "strings"

// PostgresConfig is a map-based configuration of a Postgresql connection (dbname, host etc.),
// extended with gorm-specific parameters (e.g. number of open / idle connections).
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
