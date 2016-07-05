package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsingOfEnviromentVariables(t *testing.T) {
	a := assert.New(t)

	originalArgs := os.Args
	os.Args = []string{os.Args[0]}
	defer func() { os.Args = originalArgs }()

	// given: some environment variables
	os.Setenv("GUBLE_HTTP_LISTEN", "http_listen")
	defer os.Unsetenv("GUBLE_HTTP_LISTEN")

	os.Setenv("GUBLE_LOG", "debug")
	defer os.Unsetenv("GUBLE_LOG")

	os.Setenv("GUBLE_KVS", "kvs-backend")
	defer os.Unsetenv("GUBLE_KVS")

	os.Setenv("GUBLE_STORAGE_PATH", os.TempDir())
	defer os.Unsetenv("GUBLE_STORAGE_PATH")

	os.Setenv("GUBLE_HEALTH_ENDPOINT", "health_endpoint")
	defer os.Unsetenv("GUBLE_HEALTH_ENDPOINT")

	os.Setenv("GUBLE_METRICS", "true")
	defer os.Unsetenv("GUBLE_METRICS")

	os.Setenv("GUBLE_METRICS_ENDPOINT", "metrics_endpoint")
	defer os.Unsetenv("GUBLE_METRICS_ENDPOINT")

	os.Setenv("GUBLE_MS", "ms-backend")
	defer os.Unsetenv("GUBLE_MS")

	os.Setenv("GUBLE_GCM", "true")
	defer os.Unsetenv("GUBLE_GCM")

	os.Setenv("GUBLE_GCM_API_KEY", "gcm-api-key")
	defer os.Unsetenv("GUBLE_GCM_API_KEY")

	os.Setenv("GUBLE_GCM_WORKERS", "3")
	defer os.Unsetenv("GUBLE_GCM_WORKERS")

	os.Setenv("GUBLE_NODE_ID", "1")
	defer os.Unsetenv("GUBLE_NODE_ID")

	os.Setenv("GUBLE_NODE_PORT", "10000")
	defer os.Unsetenv("GUBLE_NODE_PORT")

	os.Setenv("GUBLE_PG_HOST", "pg-host")
	defer os.Unsetenv("GUBLE_PG_HOST")

	os.Setenv("GUBLE_PG_USER", "pg-user")
	defer os.Unsetenv("GUBLE_PG_USER")

	os.Setenv("GUBLE_PG_PASSWORD", "pg-password")
	defer os.Unsetenv("GUBLE_PG_PASSWORD")

	os.Setenv("GUBLE_PG_DBNAME", "pg-dbname")
	defer os.Unsetenv("GUBLE_PG_DBNAME")

	// when we parse the arguments from environment variables
	Parse()

	// then the parsed parameters are correctly set
	assertArguments(a)
}

func TestParsingArgs(t *testing.T) {
	a := assert.New(t)

	originalArgs := os.Args

	defer func() { os.Args = originalArgs }()

	// given: a command line
	os.Args = []string{os.Args[0],
		"--http", "http_listen",
		"--log", "debug",
		"--storage-path", os.TempDir(),
		"--kvs", "kvs-backend",
		"--ms", "ms-backend",
		"--health-endpoint", "health_endpoint",
		"--metrics",
		"--metrics-endpoint", "metrics_endpoint",
		"--gcm",
		"--gcm-api-key", "gcm-api-key",
		"--gcm-workers", "3",
		"--node-id", "1",
		"--node-port", "10000",
		"--pg-host", "pg-host",
		"--pg-user", "pg-user",
		"--pg-password", "pg-password",
		"--pg-dbname", "pg-dbname",
	}

	// when we parse the arguments from command-line flags
	Parse()

	// then the parsed parameters are correctly set
	assertArguments(a)
}

func assertArguments(a *assert.Assertions) {
	a.Equal("http_listen", *HttpListen)
	a.Equal("kvs-backend", *KVS)
	a.Equal(os.TempDir(), *StoragePath)
	a.Equal("ms-backend", *MS)
	a.Equal("health_endpoint", *HealthEndpoint)

	a.Equal(true, *Metrics.Enabled)
	a.Equal("metrics_endpoint", *Metrics.Endpoint)

	a.Equal(true, *GCM.Enabled)
	a.Equal("gcm-api-key", *GCM.APIKey)
	a.Equal(3, *GCM.Workers)

	a.Equal(1, *Cluster.NodeID)
	a.Equal(10000, *Cluster.NodePort)

	a.Equal("pg-host", *Postgres.Host)
	a.Equal("pg-user", *Postgres.User)
	a.Equal("pg-password", *Postgres.Password)
	a.Equal("pg-dbname", *Postgres.DbName)

	a.Equal("debug", *Log)
}
