package config

// Config package contains the public config vars required in guble

import (
	"runtime"
	"strconv"

	log "github.com/Sirupsen/logrus"

	"gopkg.in/alecthomas/kingpin.v2"
	"net/url"
)

const (
	healthEndpointPrefix = "/_health"
)

var (
	// Listen the address to bind the HTTP server to
	Listen = kingpin.Flag("listen", "[Host:]Port the address to listen on (:8080)").
		Default(":8080").
		Short('l').
		Envar("GUBLE_LISTEN").
		String()
	// StoragePath where to save the file system store
	StoragePath = kingpin.Flag("storage-path", "The path for storing messages and key value data if 'file' is enabled (/var/lib/guble)").
			Short('p').
			Envar("GUBLE_STORAGE_PATH").
			String()
	// KVBackend sets the key-value storage format to use
	KVBackend = kingpin.Flag("kv-backend", "The storage backend for the key value store to use: file|memory (file)").
			Default("file").
			Envar("GUBLE_KV_BACKEND").
			String()
	// MSBackend sets the message store format to use
	MSBackend = kingpin.Flag("ms-backend", "The message storage backend : file|memory (file)").
			Default("file").
			Envar("GUBLE_MS_BACKEND").
			String()
	// Health sets the health endpoint to bind in the HTTP server
	Health = kingpin.Flag("health", `The health endpoint (default: /_health; value for disabling it: "")`).
		Default(healthEndpointPrefix).
		Envar("GUBLE_HEALTH_ENDPOINT").
		String()

	Metrics = struct {
		Enabled  *bool
		Endpoint *string
	}{
		// Enable metrics collection
		Enabled: kingpin.Flag("metrics", "Enable metrics").Envar("GUBLE_METRICS_ENABLED").Bool(),

		// Endpoint sets the metrics endpoint to bind in the HTTP server and return  metrics data
		Endpoint: kingpin.Flag("metrics-endpoint", "The metrics endpoint (disabled by default; a possible value for enabling it: /_metrics )").
			Envar("GUBLE_METRICS_ENDPOINT").
			String(),
	}

	// GCM settings related to activating the GCM connector module
	GCM = struct {
		Enabled *bool
		APIKey  *string
		Workers *int
	}{
		Enabled: kingpin.Flag("gcm-enabled", "Enable the Google Cloud Messaging Connector").
			Envar("GUBLE_GCM_ENABLED").
			Bool(),
		APIKey: kingpin.Flag("gcm-api-key", "The Google API Key for Google Cloud Messaging").
			Envar("GUBLE_GCM_API_KEY").
			String(),
		Workers: kingpin.Flag("gcm-workers", "The number of workers handling traffic with Google Cloud Messaging (default: GOMAXPROCS)").
			Default(strconv.Itoa(runtime.GOMAXPROCS(0))).
			Envar("GUBLE_GCM_WORKERS").
			Int(),
	}

	Cluster = struct {
		NodeID   *int
		NodePort *int
		Remotes  *[]*url.URL
	}{
		NodeID: kingpin.Flag("node-id", "This guble node's own ID (used in cluster mode): a strictly positive integer number which must be unique in cluster").
			Envar("GUBLE_NODE_ID").
			Int(),

		NodePort: kingpin.Flag("node-port", "This guble node's own local port (used in cluster mode): a strictly positive integer number").
			Default(strconv.Itoa(10000)).
			Envar("GUBLE_NODE_PORT").
			Int(),

		Remotes: kingpin.Arg("ips", "The list of IP:port of some other guble nodes (used in cluster mode)").
			URLList(),
	}

	// Log level
	Log = kingpin.Flag("log", "Log level").
		Default(log.ErrorLevel.String()).
		Envar("GUBLE_LOG").
		Enum(logLevels()...)

	parsed = false
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
	if parsed {
		return
	}
	kingpin.Parse()
	parsed = true
}
