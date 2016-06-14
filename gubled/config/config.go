package config

import (
	"github.com/alexflint/go-arg"
	"github.com/caarlos0/env"
	"runtime"
)

const (
	healthEndpointPrefix = "/_health"
)

// Config contains the fields read from command line or env variables required in guble
var (
	Config *config
)

type config struct {
	Listen string `arg:"-l,help: [Host:]Port the address to listen on (:8080)" env:"GUBLE_LISTEN"`

	StoragePath string `arg:"--storage-path,help: The path for storing messages and key value data if 'file' is enabled (/var/lib/guble)" env:"GUBLE_STORAGE_PATH"`

	KVBackend string `arg:"--kv-backend,help: The storage backend for the key value store to use: file|memory (file)" env:"GUBLE_KV_BACKEND"`
	MSBackend string `arg:"--ms-backend,help: The message storage backend : file|memory (file)" env:"GUBLE_MS_BACKEND"`

	Health  string `arg:"--health: The health endpoint (default: /_health; value for disabling it: \"\" )" env:"GUBLE_HEALTH_ENDPOINT"`
	Metrics string `arg:"--metrics: The metrics endpoint (disabled by default; a possible value for enabling it: /_metrics )" env:"GUBLE_METRICS_ENDPOINT"`

	GCM struct {
		Enabled bool   `arg:"--gcm-enable: Enable the Google Cloud Messaging Connector (false)" env:"GUBLE_GCM_ENABLE"`
		APIKey  string `arg:"--gcm-api-key: The Google API Key for Google Cloud Messaging" env:"GUBLE_GCM_API_KEY"`
		Workers int    `arg:"--gcm-workers: The number of workers handling traffic with Google Cloud Messaging (default: GOMAXPROCS)" env:"GUBLE_GCM_WORKERS"`
	}

	Log struct {
		Info  bool `arg:"--log-info,help: Log on INFO level (false)" env:"GUBLE_LOG_INFO"`
		Debug bool `arg:"--log-debug,help: Log on DEBUG level (false)" env:"GUBLE_LOG_DEBUG"`
	}
}

func init() {
	Config := &config{
		Listen:      ":8080",
		KVBackend:   "file",
		MSBackend:   "file",
		StoragePath: "/var/lib/guble",

		Health: healthEndpointPrefix,

		GCM: config.GCM{
			Workers: runtime.GOMAXPROCS(0),
		},
	}

	env.Parse(Config)
	arg.MustParse(Config)
}
