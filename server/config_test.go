package server

import (
	"github.com/stretchr/testify/assert"
	"net"
	"os"
	"testing"
)

func TestParsingOfEnvironmentVariables(t *testing.T) {
	a := assert.New(t)

	originalArgs := os.Args
	os.Args = []string{os.Args[0]}
	defer func() { os.Args = originalArgs }()

	// given: some environment variables
	os.Setenv("GUBLE_HTTP_LISTEN", "http_listen")
	defer os.Unsetenv("GUBLE_HTTP_LISTEN")

	os.Setenv("GUBLE_LOG", "debug")
	defer os.Unsetenv("GUBLE_LOG")

	os.Setenv("GUBLE_ENV", "dev")
	defer os.Unsetenv("GUBLE_ENV")

	os.Setenv("GUBLE_PROFILE", "mem")
	defer os.Unsetenv("GUBLE_PROFILE")

	os.Setenv("GUBLE_KVS", "kvs-backend")
	defer os.Unsetenv("GUBLE_KVS")

	os.Setenv("GUBLE_STORAGE_PATH", os.TempDir())
	defer os.Unsetenv("GUBLE_STORAGE_PATH")

	os.Setenv("GUBLE_HEALTH_ENDPOINT", "health_endpoint")
	defer os.Unsetenv("GUBLE_HEALTH_ENDPOINT")

	os.Setenv("GUBLE_METRICS_ENDPOINT", "metrics_endpoint")
	defer os.Unsetenv("GUBLE_METRICS_ENDPOINT")

	os.Setenv("GUBLE_PROMETHEUS_ENDPOINT", "prometheus_endpoint")
	defer os.Unsetenv("GUBLE_PROMETHEUS_ENDPOINT")

	os.Setenv("GUBLE_TOGGLES_ENDPOINT", "toggles_endpoint")
	defer os.Unsetenv("GUBLE_TOGGLES_ENDPOINT")

	os.Setenv("GUBLE_MS", "ms-backend")
	defer os.Unsetenv("GUBLE_MS")

	os.Setenv("GUBLE_WS", "true")
	defer os.Unsetenv("GUBLE_WS")

	os.Setenv("GUBLE_WS_PREFIX", "/wstream/")
	defer os.Unsetenv("GUBLE_WS_PREFIX")

	os.Setenv("GUBLE_FCM", "true")
	defer os.Unsetenv("GUBLE_FCM")

	os.Setenv("GUBLE_FCM_API_KEY", "fcm-api-key")
	defer os.Unsetenv("GUBLE_FCM_API_KEY")

	os.Setenv("GUBLE_FCM_WORKERS", "3")
	defer os.Unsetenv("GUBLE_FCM_WORKERS")

	os.Setenv("GUBLE_APNS", "true")
	defer os.Unsetenv("GUBLE_APNS")

	os.Setenv("GUBLE_APNS_PRODUCTION", "true")
	defer os.Unsetenv("GUBLE_APNS_PRODUCTION")

	os.Setenv("GUBLE_APNS_CERT_BYTES", "00ff")
	defer os.Unsetenv("GUBLE_APNS_CERT_BYTES")

	os.Setenv("GUBLE_APNS_CERT_PASSWORD", "rotten")
	defer os.Unsetenv("GUBLE_APNS_CERT_PASSWORD")

	os.Setenv("GUBLE_APNS_APP_TOPIC", "com.myapp")
	defer os.Unsetenv("GUBLE_APNS_APP_TOPIC")

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

	os.Setenv("GUBLE_NODE_REMOTES", "127.0.0.1:8080 127.0.0.1:20002")
	defer os.Unsetenv("GUBLE_NODE_REMOTES")

	os.Setenv("GUBLE_KAFKA_BROKERS", "127.0.0.1:9092 127.0.0.1:9091")
	defer os.Unsetenv("GUBLE_KAFKA_BROKERS")

	os.Setenv("GUBLE_SMS_KAFKA_TOPIC", "sms_reporting_topic")
	defer os.Unsetenv("GUBLE_SMS_KAFKA_TOPIC")

	os.Setenv("GUBLE_SMS_TOGGLEABLE", "true")
	defer os.Unsetenv("GUBLE_SMS_TOGGLEABLE")

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
		"--env", "dev",
		"--log", "debug",
		"--profile", "mem",
		"--storage-path", os.TempDir(),
		"--kvs", "kvs-backend",
		"--ms", "ms-backend",
		"--health-endpoint", "health_endpoint",
		"--metrics-endpoint", "metrics_endpoint",
		"--prometheus-endpoint", "prometheus_endpoint",
		"--toggles-endpoint", "toggles_endpoint",
		"--ws",
		"--ws-prefix", "/wstream/",
		"--fcm",
		"--fcm-api-key", "fcm-api-key",
		"--fcm-workers", "3",
		"--apns",
		"--apns-production",
		"--apns-cert-bytes", "00ff",
		"--apns-cert-password", "rotten",
		"--apns-app-topic", "com.myapp",
		"--node-id", "1",
		"--node-port", "10000",
		"--pg-host", "pg-host",
		"--pg-port", "5432",
		"--pg-user", "pg-user",
		"--pg-password", "pg-password",
		"--pg-dbname", "pg-dbname",
		"--remotes", "127.0.0.1:8080 127.0.0.1:20002",
		"--kafka-brokers", "127.0.0.1:9092 127.0.0.1:9091",
		"--sms-kafka-topic", "sms_reporting_topic",
		"--sms-toggleable",
	}

	// when we parse the arguments from command-line flags
	parseConfig()

	// then the parsed parameters are correctly set
	assertArguments(a)
}

func assertArguments(a *assert.Assertions) {
	a.Equal("http_listen", *Config.HttpListen)
	a.Equal("kvs-backend", *Config.KVS)
	a.Equal(os.TempDir(), *Config.StoragePath)
	a.Equal("ms-backend", *Config.MS)
	a.Equal("health_endpoint", *Config.HealthEndpoint)

	a.Equal("metrics_endpoint", *Config.MetricsEndpoint)
	a.Equal("prometheus_endpoint", *Config.PrometheusEndpoint)
	a.Equal("toggles_endpoint", *Config.TogglesEndpoint)

	a.Equal(true, *Config.WS.Enabled)
	a.Equal("/wstream/", *Config.WS.Prefix)

	a.Equal(true, *Config.FCM.Enabled)
	a.Equal("fcm-api-key", *Config.FCM.APIKey)
	a.Equal(3, *Config.FCM.Workers)

	a.Equal(true, *Config.APNS.Enabled)
	a.Equal(true, *Config.APNS.Production)
	a.Equal([]byte{0, 255}, *Config.APNS.CertificateBytes)
	a.Equal("rotten", *Config.APNS.CertificatePassword)
	a.Equal("com.myapp", *Config.APNS.AppTopic)

	a.Equal(uint8(1), *Config.Cluster.NodeID)
	a.Equal(10000, *Config.Cluster.NodePort)

	a.Equal("pg-host", *Config.Postgres.Host)
	a.Equal(5432, *Config.Postgres.Port)
	a.Equal("pg-user", *Config.Postgres.User)
	a.Equal("pg-password", *Config.Postgres.Password)
	a.Equal("pg-dbname", *Config.Postgres.DbName)

	a.Equal("debug", *Config.Log)
	a.Equal("dev", *Config.EnvName)
	a.Equal("mem", *Config.Profile)

	a.Equal("[127.0.0.1:9092 127.0.0.1:9091]", (*Config.KafkaProducer.Brokers).String())
	a.Equal("sms_reporting_topic", *Config.SMS.KafkaReportingTopic)

	a.Equal(true, *Config.SMS.Toggleable)

	assertClusterRemotes(a)
}

func assertClusterRemotes(a *assert.Assertions) {
	ip1, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	ip2, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:20002")
	ipList := make(tcpAddrList, 0)
	ipList = append(ipList, ip1)
	ipList = append(ipList, ip2)
	a.Equal(ipList, *Config.Cluster.Remotes)
}
