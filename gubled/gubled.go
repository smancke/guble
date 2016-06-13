package gubled

import (
	log "github.com/Sirupsen/logrus"
	"github.com/alexflint/go-arg"
	"github.com/caarlos0/env"

	"github.com/smancke/guble/gcm"
	"github.com/smancke/guble/metrics"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/server/auth"
	"github.com/smancke/guble/server/rest"
	"github.com/smancke/guble/server/webserver"
	"github.com/smancke/guble/server/websocket"
	"github.com/smancke/guble/store"

	"expvar"
	"net/url"
	"os"
	"os/signal"
	"path"
	"runtime"
	"syscall"
	"time"
)

var logger = log.WithFields(log.Fields{
	"app":    "guble",
	"module": "gubled",
	"env":    "TBD"})

const (
	healthEndpointPrefix = "/_health"
)

type Args struct {
	Listen      string   `arg:"-l,help: [Host:]Port the address to listen on (:8080)" env:"GUBLE_LISTEN"`
	LogInfo     bool     `arg:"--log-info,help: Log on INFO level (false)" env:"GUBLE_LOG_INFO"`
	LogDebug    bool     `arg:"--log-debug,help: Log on DEBUG level (false)" env:"GUBLE_LOG_DEBUG"`
	StoragePath string   `arg:"--storage-path,help: The path for storing messages and key value data if 'file' is enabled (/var/lib/guble)" env:"GUBLE_STORAGE_PATH"`
	KVBackend   string   `arg:"--kv-backend,help: The storage backend for the key value store to use: file|memory (file)" env:"GUBLE_KV_BACKEND"`
	MSBackend   string   `arg:"--ms-backend,help: The message storage backend : file|memory (file)" env:"GUBLE_MS_BACKEND"`
	GcmEnable   bool     `arg:"--gcm-enable: Enable the Google Cloud Messaging Connector (false)" env:"GUBLE_GCM_ENABLE"`
	GcmApiKey   string   `arg:"--gcm-api-key: The Google API Key for Google Cloud Messaging" env:"GUBLE_GCM_API_KEY"`
	GcmWorkers  int      `arg:"--gcm-workers: The number of workers handling traffic with Google Cloud Messaging (default: GOMAXPROCS)" env:"GUBLE_GCM_WORKERS"`
	Health      string   `arg:"--health: The health endpoint (default: /_health; value for disabling it: \"\" )" env:"GUBLE_HEALTH_ENDPOINT"`
	Metrics     string   `arg:"--metrics: The metrics endpoint (disabled by default; a possible value for enabling it: /_metrics )" env:"GUBLE_METRICS_ENDPOINT"`
	NodeId      string   `arg:"--node-id: The metrics endpoint (node id for guble node in cluster mode)" env:"GUBLE_NODE_ID"`
	NodeUrls    []string `arg:"positional,help: The list of urls in absolute form for other guble nodes in cluster mode"`
}

var ValidateStoragePath = func(args Args) error {
	if args.KVBackend == "file" || args.MSBackend == "file" {
		testfile := path.Join(args.StoragePath, "write-test-file")
		f, err := os.Create(testfile)
		if err != nil {
			logger.WithFields(log.Fields{
				"storagePath": args.StoragePath,
				"err":         err,
			}).Error("Storage path not present/writeable.")

			if args.StoragePath == "/var/lib/guble" {
				logger.WithFields(log.Fields{
					"storagePath": args.StoragePath,
					"err":         err,
				}).Error("Use --storage-path=<path> to override the default location, or create the directory with RW rights.")
			}
			return err
		}
		f.Close()
		os.Remove(testfile)
	}
	return nil
}

var CreateKVStore = func(args Args) store.KVStore {
	switch args.KVBackend {
	case "memory":
		return store.NewMemoryKVStore()
	case "file":
		db := store.NewSqliteKVStore(path.Join(args.StoragePath, "kv-store.db"), true)
		if err := db.Open(); err != nil {
			logger.WithField("err", err).Panic("Could not open db connection")

		}
		return db
	default:
		panic(fmt.Errorf("Unknown key-value backend: %q", args.KVBackend))
	}
}

var CreateMessageStore = func(args Args) store.MessageStore {
	switch args.MSBackend {
	case "none", "":
		return store.NewDummyMessageStore(store.NewMemoryKVStore())
	case "file":
		logger.WithField("storagePath", args.StoragePath).Info("Using FileMessageStore in directory")

		return store.NewFileMessageStore(args.StoragePath)
	default:
		panic(fmt.Errorf("Unknown message-store backend: %q", args.MSBackend))
	}
}

var CreateModules = func(router server.Router, args Args) []interface{} {
	modules := make([]interface{}, 0, 3)

	if wsHandler, err := websocket.NewWSHandler(router, "/stream/"); err != nil {
		logger.WithField("err", err).Error("Error loading WSHandler module:")
	} else {
		modules = append(modules, wsHandler)
	}

	modules = append(modules, rest.NewRestMessageAPI(router, "/api/"))

	if args.GcmEnable {
		if args.GcmApiKey == "" {
			logger.Panic("GCM API Key has to be provided, if GCM is enabled")

		}

		logger.Info("Google cloud messaging: enabled")

		logger.WithField("args.GcmWOrkers", args.GcmWorkers).Debug("Workers")

		if gcm, err := gcm.NewGCMConnector(router, "/gcm/", args.GcmApiKey, args.GcmWorkers); err != nil {
			logger.WithField("err", err).Error("Error loading GCMConnector:")
		} else {
			modules = append(modules, gcm)
		}
	} else {
		logger.Info("Google cloud messaging: disabled")
	}

	return modules
}

func Main() {
	defer func() {
		if p := recover(); p != nil {
			logger.Fatal("Fatal error in gubled after recover")
		}
	}()

	args := loadArgs()
	if args.LogInfo {
		log.SetLevel(log.InfoLevel)
	}
	if args.LogDebug {
		log.SetLevel(log.DebugLevel)
	}

	if err := ValidateStoragePath(args); err != nil {
		logger.Fatal("Fatal error in gubled in validation for storage path")
	}

	BenchmarkCluster(40, 30*time.Second, 17000)

	service := StartService(args)

	waitForTermination(func() {
		err := service.Stop()
		if err != nil {
			logger.WithField("err", err).Error("Error when stopping service")
		}
	})
}

func validateURLs(values []string) []string {
	correctURLs := make([]string, 0)
	for _, posibleUrl := range values {

		u, err := url.Parse(posibleUrl)
		if err == nil && u.IsAbs() {
			correctURLs = append(correctURLs, u.Host)
		}
	}
	return correctURLs
}

func StartService(args Args) *server.Service {
	accessManager := auth.NewAllowAllAccessManager(true)
	messageStore := CreateMessageStore(args)
	kvStore := CreateKVStore(args)

	router := server.NewRouter(accessManager, messageStore, kvStore)
	webserver := webserver.New(args.Listen)

	service := server.NewService(router, webserver).
		HealthEndpointPrefix(args.Health).
		MetricsEndpointPrefix(args.Metrics).
		GubleNodeID(args.NodeId).
		GubleNodesURLs(validateURLs(args.NodeUrls))

	service.RegisterModules(CreateModules(router, args)...)

	if err := service.Start(); err != nil {
		if err := service.Stop(); err != nil {
			logger.WithField("err", err).Error("Error when stopping service after Start() failed")
		}
		logger.WithField("err", err).Fatal("Service could not be started")
	}
	expvar.Publish("guble.args", expvar.Func(func() interface{} {
		return args
	}))

	return service
}

func loadArgs() Args {
	args := Args{
		Listen:      ":8080",
		KVBackend:   "file",
		MSBackend:   "file",
		StoragePath: "/var/lib/guble",
		GcmWorkers:  runtime.GOMAXPROCS(0),
		Health:      healthEndpointPrefix,
	}

	env.Parse(&args)
	arg.MustParse(&args)
	return args
}

func waitForTermination(callback func()) {
	signalC := make(chan os.Signal)
	signal.Notify(signalC, syscall.SIGINT, syscall.SIGTERM)

	logger.Infof("Got signal '%v' .. exiting gracefully now", <-signalC)

	callback()
	metrics.LogOnDebugLevel()
	logger.Info("Exit gracefully now")
	os.Exit(0)
}
