package server

import (
	"github.com/Bogh/gcm"
	log "github.com/Sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"
)

const (
	defaultHttpListen     = ":8080"
	defaultHealthEndpoint = "/_health"
	defaultKVSBackend     = "file"
	defaultMSBackend      = "file"
	defaultStoragePath    = "/var/lib/guble"
	defaultNodePort       = "10000"
)

var (
	defaultGCMEndpoint = gcm.GcmSendEndpoint
)

type (
	// PostgresConfig is used for configuring the Postgresql connection.
	PostgresConfig struct {
		Host     *string
		Port     *int
		User     *string
		Password *string
		DbName   *string
	}
	// GCMConfig is used for configuring the Google Cloud Messaging component.
	GCMConfig struct {
		Enabled  *bool
		APIKey   *string
		Workers  *int
		Endpoint *string
	}
	// ClusterConfig is used for configuring the cluster component.
	ClusterConfig struct {
		NodeID   *uint8
		NodePort *int
		Remotes  *tcpAddrList
	}
	// Config is used for configuring Guble (including its components).
	Config struct {
		Log             *string
		EnvName         *string
		HttpListen      *string
		KVS             *string
		MS              *string
		StoragePath     *string
		HealthEndpoint  *string
		MetricsEndpoint *string
		Postgres        PostgresConfig
		GCM             GCMConfig
		Cluster         ClusterConfig
	}
)

var (
	parsed = false
	config = &Config{
		Log: kingpin.Flag("log", "Log level").
			Default(log.ErrorLevel.String()).
			Envar("GUBLE_LOG").
			Enum(logLevels()...),
		EnvName: kingpin.Flag("env", `Name of the environment on which the application is running`).
			Default(development).
			Envar("GUBLE_ENV_NAME").
			Enum(environments...),
		HttpListen: kingpin.Flag("http", `The address to for the HTTP server to listen on (format: "[Host]:Port")`).
			Default(defaultHttpListen).
			Envar("GUBLE_HTTP_LISTEN").
			String(),
		KVS: kingpin.Flag("kvs", "The storage backend for the key-value store to use : file | memory").
			Default(defaultKVSBackend).
			HintOptions("file", "memory").
			Envar("GUBLE_KVS").
			String(),
		MS: kingpin.Flag("ms", "The message storage backend : file | memory").
			Default(defaultMSBackend).
			HintOptions("file", "memory").
			Envar("GUBLE_MS").
			String(),
		StoragePath: kingpin.Flag("storage-path", "The path for storing messages and key-value data if 'file' is selected").
			Default(defaultStoragePath).
			Envar("GUBLE_STORAGE_PATH").
			ExistingDir(),
		HealthEndpoint: kingpin.Flag("health-endpoint", `The health endpoint to be used by the HTTP server (value for disabling it: "")`).
			Default(defaultHealthEndpoint).
			Envar("GUBLE_HEALTH_ENDPOINT").
			String(),
		MetricsEndpoint: kingpin.Flag("metrics-endpoint", `The metrics endpoint to be used by the HTTP server (disabled by default; a possible value for enabling it: "/_metrics" )`).
			Default("").
			Envar("GUBLE_METRICS_ENDPOINT").
			String(),
		Postgres: PostgresConfig{
			Host:     kingpin.Flag("pg-host", "The PostgreSQL hostname").Default("localhost").Envar("GUBLE_PG_HOST").String(),
			Port:     kingpin.Flag("pg-port", "The PostgreSQL port").Default("5432").Envar("GUBLE_PG_PORT").Int(),
			User:     kingpin.Flag("pg-user", "The PostgreSQL user").Default("guble").Envar("GUBLE_PG_USER").String(),
			Password: kingpin.Flag("pg-password", "The PostgreSQL password").Default("guble").Envar("GUBLE_PG_PASSWORD").String(),
			DbName:   kingpin.Flag("pg-dbname", "The PostgreSQL database name").Default("guble").Envar("GUBLE_PG_DBNAME").String(),
		},
		GCM: GCMConfig{
			Enabled: kingpin.Flag("gcm", "Enable the Google Cloud Messaging Connector").
				Envar("GUBLE_GCM").Bool(),
			APIKey: kingpin.Flag("gcm-api-key", "The Google API Key for Google Cloud Messaging").
				Envar("GUBLE_GCM_API_KEY").String(),
			Workers: kingpin.Flag("gcm-workers", "The number of workers handling traffic with Google Cloud Messaging (default: GOMAXPROCS)").
				Default(strconv.Itoa(runtime.GOMAXPROCS(0))).Envar("GUBLE_GCM_WORKERS").Int(),
			Endpoint: kingpin.Flag("gcm-endpoint", "The Google Cloud Messaging endpoint").
				Default(defaultGCMEndpoint).
				Envar("GUBLE_GCM_ENDPOINT").String(),
		},
		Cluster: ClusterConfig{
			NodeID: kingpin.Flag("node-id", "(cluster mode) This guble node's own ID: a strictly positive integer number which must be unique in cluster").
				Envar("GUBLE_NODE_ID").Uint8(),
			NodePort: kingpin.Flag("node-port", "(cluster mode) This guble node's own local port: a strictly positive integer number").
				Default(defaultNodePort).Envar("GUBLE_NODE_PORT").Int(),
			Remotes: tcpAddrListParser(kingpin.Flag("remotes", `(cluster mode) The list of TCP addresses of some other guble nodes (format: "IP:port")`).
				Envar("GUBLE_NODE_REMOTES")),
		},
	}
)

func logLevels() (levels []string) {
	for _, level := range log.AllLevels {
		levels = append(levels, level.String())
	}
	return
}

const (
	development   string = "dev"
	integration   string = "int"
	preproduction string = "pre"
	production    string = "prod"
)

var environments []string = []string{development, integration, preproduction, production}

// parseConfig parses the flags from command line. Must be used before accessing the config.
// If there are missing or invalid arguments it will exit the application
// and display a message.
func parseConfig() {
	if parsed {
		return
	}
	kingpin.Parse()
	parsed = true
	return
}

type tcpAddrList []*net.TCPAddr

func (h *tcpAddrList) Set(value string) error {
	addresses := strings.Split(value, " ")
	for _, addr := range addresses {
		logger.WithField("addr", addr).Info("value")
		parts := strings.SplitN(addr, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("expected HEADER:VALUE got '%s'", addr)
		}
		addr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return err
		}
		*h = append(*h, addr)
	}
	return nil
}

func tcpAddrListParser(s kingpin.Settings) (target *tcpAddrList) {
	slist := make(tcpAddrList, 0)
	s.SetValue(&slist)
	return &slist
}

func (h *tcpAddrList) String() string {
	return ""
}
