package gubled

import (
	"fmt"

	"github.com/smancke/guble/gubled/config"

	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/gcm"
	"github.com/smancke/guble/metrics"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/server/auth"
	"github.com/smancke/guble/server/rest"
	"github.com/smancke/guble/server/webserver"
	"github.com/smancke/guble/server/websocket"
	"github.com/smancke/guble/store"

	"os"
	"os/signal"
	"path"
	"syscall"
)

var logger = log.WithFields(log.Fields{
	"service":          "guble",
	"application_type": "service",
	"log_type":         "application",
	"module":           "gubled",
	"environment":      "TBD"})

var ValidateStoragePath = func() error {
	if *config.KVBackend == "file" || *config.MSBackend == "file" {
		testfile := path.Join(*config.StoragePath, "write-test-file")
		f, err := os.Create(testfile)
		if err != nil {
			logger.WithFields(log.Fields{
				"storagePath": *config.StoragePath,
				"err":         err,
			}).Error("Storage path not present/writeable.")

			if *config.StoragePath == "/var/lib/guble" {
				logger.WithFields(log.Fields{
					"storagePath": *config.StoragePath,
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

var CreateKVStore = func() store.KVStore {
	switch *config.KVBackend {
	case "memory":
		return store.NewMemoryKVStore()
	case "file":
		db := store.NewSqliteKVStore(path.Join(*config.StoragePath, "kv-store.db"), true)
		if err := db.Open(); err != nil {
			logger.WithField("err", err).Panic("Could not open db connection")

		}
		return db
	default:
		panic(fmt.Errorf("Unknown key-value backend: %q", *config.KVBackend))
	}
}

var CreateMessageStore = func() store.MessageStore {
	switch *config.MSBackend {
	case "none", "":
		return store.NewDummyMessageStore(store.NewMemoryKVStore())
	case "file":
		logger.WithField("storagePath", *config.StoragePath).Info("Using FileMessageStore in directory")

		return store.NewFileMessageStore(*config.StoragePath)
	default:
		panic(fmt.Errorf("Unknown message-store backend: %q", *config.MSBackend))
	}
}

var CreateModules = func(router server.Router) []interface{} {
	modules := make([]interface{}, 0, 3)

	if wsHandler, err := websocket.NewWSHandler(router, "/stream/"); err != nil {
		logger.WithField("err", err).Error("Error loading WSHandler module:")
	} else {
		modules = append(modules, wsHandler)
	}

	modules = append(modules, rest.NewRestMessageAPI(router, "/api/"))

	if *config.GCM.Enabled {
		if *config.GCM.APIKey == "" {
			logger.Panic("GCM API Key has to be provided, if GCM is enabled")

		}

		logger.Info("Google cloud messaging: enabled")

		logger.WithField("count", *config.GCM.Workers).Debug("GCM workers")

		if gcm, err := gcm.NewGCMConnector(router, "/gcm/", *config.GCM.APIKey, *config.GCM.Workers); err != nil {
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
	config.Parse()
	defer func() {
		if p := recover(); p != nil {
			logger.Fatal("Fatal error in gubled after recover")
		}
	}()

	// set log level
	level, err := log.ParseLevel(*config.Log)
	if err != nil {
		logger.WithField("error", err).Fatal("Invalid log level")
	}
	log.SetLevel(level)

	if err := ValidateStoragePath(); err != nil {
		logger.Fatal("Fatal error in gubled in validation for storage path")
	}

	service := StartService()

	waitForTermination(func() {
		err := service.Stop()
		if err != nil {
			logger.WithField("err", err).Error("Error when stopping service")
		}
	})
}

func StartService() *server.Service {
	accessManager := auth.NewAllowAllAccessManager(true)
	messageStore := CreateMessageStore()
	kvStore := CreateKVStore()

	router := server.NewRouter(accessManager, messageStore, kvStore)
	webserver := webserver.New(*config.Listen)

	service := server.NewService(router, webserver).
		HealthEndpointPrefix(*config.Health).
		MetricsEndpointPrefix(*config.Metrics.Endpoint)

	service.RegisterModules(CreateModules(router)...)

	if err := service.Start(); err != nil {
		if err := service.Stop(); err != nil {
			logger.WithField("err", err).Error("Error when stopping service after Start() failed")
		}
		logger.WithField("err", err).Fatal("Service could not be started")
	}

	return service
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
