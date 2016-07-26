package server

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

	os.Setenv("GUBLE_PG_PORT", "5432")
	defer os.Unsetenv("GUBLE_PG_PORT")

	os.Setenv("GUBLE_PG_USER", "pg-user")
	defer os.Unsetenv("GUBLE_PG_USER")

	os.Setenv("GUBLE_PG_PASSWORD", "pg-password")
	defer os.Unsetenv("GUBLE_PG_PASSWORD")

	os.Setenv("GUBLE_PG_DBNAME", "pg-dbname")
	defer os.Unsetenv("GUBLE_PG_DBNAME")

	// when we parse the arguments from environment variables
	parseConfig()

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
		"--metrics-endpoint", "metrics_endpoint",
		"--gcm",
		"--gcm-api-key", "gcm-api-key",
		"--gcm-workers", "3",
		"--node-id", "1",
		"--node-port", "10000",
		"--pg-host", "pg-host",
		"--pg-port", "5432",
		"--pg-user", "pg-user",
		"--pg-password", "pg-password",
		"--pg-dbname", "pg-dbname",
	}

	// when we parse the arguments from command-line flags
	parseConfig()

	// then the parsed parameters are correctly set
	assertArguments(a)
}

func assertArguments(a *assert.Assertions) {
	a.Equal("http_listen", *config.HttpListen)
	a.Equal("kvs-backend", *config.KVS)
	a.Equal(os.TempDir(), *config.StoragePath)
	a.Equal("ms-backend", *config.MS)
	a.Equal("health_endpoint", *config.HealthEndpoint)

	a.Equal("metrics_endpoint", *config.MetricsEndpoint)

	a.Equal(true, *config.GCM.Enabled)
	a.Equal("gcm-api-key", *config.GCM.APIKey)
	a.Equal(3, *config.GCM.Workers)

	a.Equal(1, *config.Cluster.NodeID)
	a.Equal(10000, *config.Cluster.NodePort)

	a.Equal("pg-host", *config.Postgres.Host)
	a.Equal(5432, *config.Postgres.Port)
	a.Equal("pg-user", *config.Postgres.User)
	a.Equal("pg-password", *config.Postgres.Password)
	a.Equal("pg-dbname", *config.Postgres.DbName)

	a.Equal("debug", *config.Log)
}
