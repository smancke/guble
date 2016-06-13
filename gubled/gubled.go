package gubled

import (
	log "github.com/Sirupsen/logrus"
	"github.com/alexflint/go-arg"
	"github.com/caarlos0/env"
	"github.com/hashicorp/memberlist"

	"github.com/smancke/guble/gcm"
	"github.com/smancke/guble/metrics"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/server/auth"
	"github.com/smancke/guble/server/rest"
	"github.com/smancke/guble/server/webserver"
	"github.com/smancke/guble/server/websocket"
	"github.com/smancke/guble/store"

	"expvar"
	"fmt"
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
	Listen      string `arg:"-l,help: [Host:]Port the address to listen on (:8080)" env:"GUBLE_LISTEN"`
	LogInfo     bool   `arg:"--log-info,help: Log on INFO level (false)" env:"GUBLE_LOG_INFO"`
	LogDebug    bool   `arg:"--log-debug,help: Log on DEBUG level (false)" env:"GUBLE_LOG_DEBUG"`
	StoragePath string `arg:"--storage-path,help: The path for storing messages and key value data if 'file' is enabled (/var/lib/guble)" env:"GUBLE_STORAGE_PATH"`
	KVBackend   string `arg:"--kv-backend,help: The storage backend for the key value store to use: file|memory (file)" env:"GUBLE_KV_BACKEND"`
	MSBackend   string `arg:"--ms-backend,help: The message storage backend : file|memory (file)" env:"GUBLE_MS_BACKEND"`
	GcmEnable   bool   `arg:"--gcm-enable: Enable the Google Cloud Messaging Connector (false)" env:"GUBLE_GCM_ENABLE"`
	GcmApiKey   string `arg:"--gcm-api-key: The Google API Key for Google Cloud Messaging" env:"GUBLE_GCM_API_KEY"`
	GcmWorkers  int    `arg:"--gcm-workers: The number of workers handling traffic with Google Cloud Messaging (default: GOMAXPROCS)" env:"GUBLE_GCM_WORKERS"`
	Health      string `arg:"--health: The health endpoint (default: /_health; value for disabling it: \"\" )" env:"GUBLE_HEALTH_ENDPOINT"`
	Metrics     string `arg:"--metrics: The metrics endpoint (disabled by default; a possible value for enabling it: /_metrics )" env:"GUBLE_METRICS_ENDPOINT"`
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

	startClusterBenchmark(32, 2*time.Second)

	service := StartService(args)

	waitForTermination(func() {
		err := service.Stop()
		if err != nil {
			logger.WithField("err", err).Error("Error when stopping service")
		}
	})
}

func startClusterBenchmark(num int, timeoutForCheck time.Duration) {
	var members []*memberlist.Memberlist

	eventC := make(chan memberlist.NodeEvent, num)

	addr := "127.0.0.1"
	var firstMemberName string
	for i := 0; i < num; i++ {
		c := memberlist.DefaultLANConfig()
		c.Name = fmt.Sprintf("%s:%d", addr, 12345+i)
		c.BindAddr = addr
		c.BindPort = 12345 + i
		c.ProbeInterval = 20 * time.Millisecond
		c.ProbeTimeout = 100 * time.Millisecond
		c.GossipInterval = 20 * time.Millisecond
		c.PushPullInterval = 200 * time.Millisecond
		c.LogOutput = logger.Logger.Out
		c.Logger = nil

		if i == 0 {
			c.Events = &memberlist.ChannelEventDelegate{eventC}
		}

		newMemberList, err := memberlist.Create(c)
		if err != nil {
			//TODO Cosmin log properly
			//t.Fatalf("unexpected err: %s", err)
		}
		members = append(members, newMemberList)
		defer newMemberList.Shutdown()

		if i == 0 {
			firstMemberName = c.Name
		} else {
			num, err := newMemberList.Join([]string{firstMemberName})
			if num == 0 || err != nil {
				log.WithField("error", err).Fatal("Unexpected fatal error when joining the cluster")
			}
		}
	}

	// Wait and print debug info
	breakTimer := time.After(timeoutForCheck)
WAIT:
	for {
		select {
		case e := <-eventC:
			lwf := log.WithFields(log.Fields{
				"node":       *e.Node,
				"numMembers": members[0].NumMembers(),
			})
			if e.Event == memberlist.NodeJoin {
				lwf.Debug("Node join")
			} else {
				lwf.Debug("Node leave")
			}
		case <-breakTimer:
			break WAIT
		}
	}

	convergence := true
	for idx, m := range members {
		actualNum := m.NumMembers()
		if actualNum != num {
			convergence = false
			log.WithFields(log.Fields{
				"index":    idx,
				"expected": num,
				"actual":   actualNum,
			}).Error("wrong number of members")
		}
	}
	if convergence {
		log.Info("correct number of members")
	}

}

func StartService(args Args) *server.Service {
	accessManager := auth.NewAllowAllAccessManager(true)
	messageStore := CreateMessageStore(args)
	kvStore := CreateKVStore(args)

	router := server.NewRouter(accessManager, messageStore, kvStore)
	webserver := webserver.New(args.Listen)

	service := server.NewService(router, webserver).HealthEndpointPrefix(args.Health).MetricsEndpointPrefix(args.Metrics)

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
