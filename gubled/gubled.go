package gubled

import (
	log "github.com/Sirupsen/logrus"
	"github.com/alexflint/go-arg"
	"github.com/caarlos0/env"

	"github.com/smancke/guble/gcm"
	"github.com/smancke/guble/metrics"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/server/auth"
	"github.com/smancke/guble/server/cluster"
	"github.com/smancke/guble/server/rest"
	"github.com/smancke/guble/server/webserver"
	"github.com/smancke/guble/server/websocket"
	"github.com/smancke/guble/store"

	"expvar"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"syscall"
)

var logger = log.WithFields(log.Fields{
	"app":    "guble",
	"module": "gubled",
	"env":    "TBD"})

const (
	defaultHttpListen     = ":8080"
	defaultKVBackend      = "file"
	defaultMSBackend      = "file"
	defaultStoragePath    = "/var/lib/guble"
	defaultHealthEndpoint = "/_health"
	defaultNodePort       = 10000
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
	NodeID      int      `arg:"--node-id: This guble node's own ID (used in cluster mode): a strictly positive integer number which must be unique in cluster" env:"GUBLE_NODE_ID"`
	NodePort    int      `arg:"--node-port: This guble node's own local port (used in cluster mode): a strictly positive integer number" env:"GUBLE_NODE_PORT"`
	Remotes     []string `arg:"positional,help: The list of URLs in absolute form of some other guble nodes (used in cluster mode)"`
}

var ValidateStoragePath = func(args Args) error {
	if args.KVBackend == "file" || args.MSBackend == "file" {
		testfile := path.Join(args.StoragePath, "write-test-file")
		f, err := os.Create(testfile)
		if err != nil {
			logger.WithFields(log.Fields{
				"storagePath": args.StoragePath,
				"error":       err,
			}).Error("Storage path not present/writeable.")

			if args.StoragePath == "/var/lib/guble" {
				logger.WithFields(log.Fields{
					"storagePath": args.StoragePath,
					"error":       err,
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
			logger.WithField("error", err).Panic("Could not open database connection")
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
		logger.WithField("error", err).Error("Error loading WSHandler module:")
	} else {
		modules = append(modules, wsHandler)
	}

	modules = append(modules, rest.NewRestMessageAPI(router, "/api/"))

	if args.GcmEnable {
		if args.GcmApiKey == "" {
			logger.Panic("GCM API Key has to be provided, if GCM is enabled")
		}

		logger.Info("Google cloud messaging: enabled")
		logger.WithField("args.GcmWorkers", args.GcmWorkers).Debug("GCM Workers")

		if gcm, err := gcm.NewGCMConnector(router, "/gcm/", args.GcmApiKey, args.GcmWorkers); err != nil {
			logger.WithField("error", err).Error("Error loading GCMConnector:")
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
		logger.Fatal("Fatal error in gubled when validating the storage path")
	}

	service := StartService(args)

	waitForTermination(func() {
		err := service.Stop()
		if err != nil {
			logger.WithField("error", err).Error("Error when stopping service")
		}
	})
}

func StartService(args Args) *server.Service {
	accessManager := auth.NewAllowAllAccessManager(true)
	messageStore := CreateMessageStore(args)
	kvStore := CreateKVStore(args)

	var c *cluster.Cluster
	if args.NodeID > 0 {
		validRemotes := validateCluster(args.NodeID, args.NodePort, args.Remotes)
		logger.Info("Starting in cluster-mode")
		clusterConfig := &cluster.ClusterConfig{
			ID:      args.NodeID,
			Port:    args.NodePort,
			Remotes: validRemotes,
		}
		c = cluster.NewCluster(clusterConfig)
	} else {
		logger.Info("Starting in standalone-mode")
	}

	router := server.NewRouter(accessManager, messageStore, kvStore, c)
	webserver := webserver.New(args.Listen)

	service := server.NewService(router, webserver).
		HealthEndpoint(args.Health).
		MetricsEndpoint(args.Metrics)

	service.RegisterModules(CreateModules(router, args)...)

	if err := service.Start(); err != nil {
		if err := service.Stop(); err != nil {
			logger.WithField("error", err).Error("Error when stopping service after Start() failed")
		}
		logger.WithField("error", err).Fatal("Service could not be started")
	}
	expvar.Publish("guble.args", expvar.Func(func() interface{} {
		return args
	}))

	return service
}

func loadArgs() Args {
	args := Args{
		Listen:      defaultHttpListen,
		KVBackend:   defaultKVBackend,
		MSBackend:   defaultMSBackend,
		StoragePath: defaultStoragePath,
		GcmWorkers:  runtime.GOMAXPROCS(0),
		Health:      defaultHealthEndpoint,
		NodePort:    defaultNodePort,
	}

	env.Parse(&args)
	arg.MustParse(&args)
	return args
}

func validateCluster(nodeID int, nodePort int, potentialRemotes []string) []string {
	validRemotes := validateRemoteHostsWithPorts(potentialRemotes)
	if (nodeID <= 0 && len(validRemotes) > 0) || (nodePort <= 0) {
		errorMessage := "Could not start in cluster-mode: invalid/incomplete parameters"
		logger.WithFields(log.Fields{
			"nodeID":               nodeID,
			"nodePort":             nodePort,
			"numberOfValidRemotes": len(validRemotes),
		}).Fatal(errorMessage)
	}
	return validRemotes
}

func validateRemoteHostsWithPorts(potentialRemotes []string) []string {
	var validRemotes []string
	for _, potentialRemote := range potentialRemotes {
		url, err := url.Parse(strings.Join([]string{"http://", potentialRemote}, ""))
		if err == nil {
			validRemotes = append(validRemotes, url.Host)
		}
	}
	logger.WithField("validRemotes", validRemotes).Debug("List of valid Remotes (hosts with ports)")
	return validRemotes
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
