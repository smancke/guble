package config

// Config package contains the public config vars required in guble

import (
	"runtime"
	"strconv"

	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	healthEndpointPrefix = "/_health"
)

var (
	Listen = kingpin.Flag("listen", "[Host:]Port the address to listen on (:8080)").
		Default(":8080").
		Short('l').
		Envar("GUBLE_LISTEN").
		String()
	StoragePath = kingpin.Flag("storage-path", "The path for storing messages and key value data if 'file' is enabled (/var/lib/guble)").
			Short('p').
			Envar("GUBLE_STORAGE_PATH").
			String()
	KVBackend = kingpin.Flag("kv-backend", "The storage backend for the key value store to use: file|memory (file)").
			Default("file").
			Envar("GUBLE_KV_BACKEND").
			String()
	MSBackend = kingpin.Flag("ms-backend", "The message storage backend : file|memory (file)").
			Default("file").
			Envar("GUBLE_MS_BACKEND").
			String()
	Health = kingpin.Flag("health", `The health endpoint (default: /_health; value for disabling it: "")`).
		Default(healthEndpointPrefix).
		Envar("GUBLE_HEALTH_ENDPOINT").
		String()
	Metrics = kingpin.Flag("metrics", "The metrics endpoint (disabled by default; a possible value for enabling it: /_metrics )").
		Envar("GUBLE_METRICS_ENDPOINT").
		String()

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

	Log = struct {
		Info, Debug *bool
	}{
		Info: kingpin.Flag("log-info", "Log on INFO level (false)").
			Default("false").
			Envar("GUBLE_LOG_INFO").
			Bool(),
		Debug: kingpin.Flag("log-debug", "Log on debug level (false)").
			Default("false").
			Envar("GUBLE_LOG_DEBUG").
			Bool(),
	}
)

func init() {
	kingpin.Parse()
}
