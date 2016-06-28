package config

// Config package contains the public config vars required in guble

import (
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"net"
	"runtime"
	"strconv"
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
	HttpListen = kingpin.Flag("http", `The address to for the HTTP server to listen on (format: "[Host]:Port")`).
			Default(defaultHttpListen).Envar("GUBLE_HTTP_LISTEN").String()
	KVS = kingpin.Flag("kvs", "The storage backend for the key-value store to use : file | memory").
		Default(defaultKVSBackend).HintOptions([]string{"file", "memory"}...).Envar("GUBLE_KVS").String()
	MS = kingpin.Flag("ms", "The message storage backend : file | memory").
		Default(defaultMSBackend).HintOptions([]string{"file", "memory"}...).Envar("GUBLE_MS").String()
	StoragePath = kingpin.Flag("storage-path", "The path for storing messages and key-value data if 'file' is selected").
			Default(defaultStoragePath).Envar("GUBLE_STORAGE_PATH").ExistingDir()
	HealthEndpoint = kingpin.Flag("health-endpoint", `The health endpoint to be used by the HTTP server (value for disabling it: "")`).
			Default(defaultHealthEndpoint).Envar("GUBLE_HEALTH_ENDPOINT").String()

	Metrics = struct {
		Enabled  *bool
		Endpoint *string
	}{
		Enabled: kingpin.Flag("metrics", "Enable collection of metrics").Envar("GUBLE_METRICS").Bool(),
		Endpoint: kingpin.Flag("metrics-endpoint", `The metrics endpoint to be used by the HTTP server (disabled by default; a possible value for enabling it: "/_metrics" )`).
			Default("").Envar("GUBLE_METRICS_ENDPOINT").String(),
	}

	// GCM settings related to activating the GCM connector module
	GCM = struct {
		Enabled *bool
		APIKey  *string
		Workers *int
	}{
		Enabled: kingpin.Flag("gcm", "Enable the Google Cloud Messaging Connector").
			Envar("GUBLE_GCM").Bool(),
		APIKey: kingpin.Flag("gcm-api-key", "The Google API Key for Google Cloud Messaging").
			Envar("GUBLE_GCM_API_KEY").String(),
		Workers: kingpin.Flag("gcm-workers", "The number of workers handling traffic with Google Cloud Messaging (default: GOMAXPROCS)").
			Default(strconv.Itoa(runtime.GOMAXPROCS(0))).Envar("GUBLE_GCM_WORKERS").Int(),
	}

	// Cluster settings related to activating the cluster module
	Cluster = struct {
		NodeID   *int
		NodePort *int
		Remotes  *[]*net.TCPAddr
	}{
		NodeID: kingpin.Flag("node-id", "(cluster mode) This guble node's own ID: a strictly positive integer number which must be unique in cluster").
			Envar("GUBLE_NODE_ID").Int(),
		NodePort: kingpin.Flag("node-port", "(cluster mode) This guble node's own local port: a strictly positive integer number").
			Default(defaultNodePort).Envar("GUBLE_NODE_PORT").Int(),
		Remotes: kingpin.Arg("tcplist", `(cluster mode) The list of TCP addresses of some other guble nodes (format: "IP:port")`).
			TCPList(),
	}

	// Log level
	Log = kingpin.Flag("log", "Log level").
		Default(log.ErrorLevel.String()).
		Envar("GUBLE_LOG").
		Enum(logLevels()...)
)

func logLevels() (levels []string) {
	for _, level := range log.AllLevels {
		levels = append(levels, level.String())
	}
	return
}

// Parse parses the flags from command line. Must be used before accessing the config.
// If there are missing or invalid arguments it will exit the application and display a
// corresponding message
func Parse() {
	kingpin.Parse()
}

func init() {
	// skip init if in test mode
	if strings.HasSuffix(os.Args[0], ".test") {
		return
	}
	Parse()
}
