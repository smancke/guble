package gubled

import (
	"fmt"

	"github.com/smancke/guble/gubled/config"

	log "github.com/Sirupsen/logrus"

	"github.com/smancke/guble/gcm"
	"github.com/smancke/guble/metrics"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/server/auth"
	"github.com/smancke/guble/server/cluster"
	"github.com/smancke/guble/server/rest"
	"github.com/smancke/guble/server/webserver"
	"github.com/smancke/guble/server/websocket"
	"github.com/smancke/guble/store"

	"net"
	"os"
	"os/signal"
	"path"
	"syscall"
)

const (
	fileOption = "file"
)

var ValidateStoragePath = func() error {
	if *config.KVS == fileOption || *config.MS == fileOption {
		testfile := path.Join(*config.StoragePath, "write-test-file")
		f, err := os.Create(testfile)
		if err != nil {
			logger.WithFields(log.Fields{
				"storagePath": *config.StoragePath,
				"err":         err,
			}).Error("Storage path not present/writeable.")

			return err
		}
		f.Close()
		os.Remove(testfile)
	}
	return nil
}

var CreateKVStore = func() store.KVStore {
	switch *config.KVS {
	case "memory":
		return store.NewMemoryKVStore()
	case "file":
		db := store.NewSqliteKVStore(path.Join(*config.StoragePath, "kv-store.db"), true)
		if err := db.Open(); err != nil {
			logger.WithField("err", err).Panic("Could not open database connection")
		}
		return db
	default:
		panic(fmt.Errorf("Unknown key-value backend: %q", *config.KVS))
	}
}

var CreateMessageStore = func() store.MessageStore {
	switch *config.MS {
	case "none", "":
		return store.NewDummyMessageStore(store.NewMemoryKVStore())
	case "file":
		logger.WithField("storagePath", *config.StoragePath).Info("Using FileMessageStore in directory")

		return store.NewFileMessageStore(*config.StoragePath)
	default:
		panic(fmt.Errorf("Unknown message-store backend: %q", *config.MS))
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
		logger.Info("Google cloud messaging: enabled")

		if *config.GCM.APIKey == "" {
			logger.Panic("GCM API Key has to be provided, if GCM is enabled")
		}

		logger.WithField("count", *config.GCM.Workers).Debug("GCM workers")

		if gcm, err := gcm.New(router, "/gcm/", *config.GCM.APIKey, *config.GCM.Workers); err != nil {
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
		logger.Fatal("Fatal error in gubled in validation of storage path")
	}

	service := StartService()

	waitForTermination(func() {
		err := service.Stop()
		if err != nil {
			logger.WithField("err", err).Error("Error when stopping service")
		}
	})
}

// TODO: StartService should return an error in case it fails to start
func StartService() *server.Service {
	accessManager := auth.NewAllowAllAccessManager(true)
	messageStore := CreateMessageStore()
	kvStore := CreateKVStore()

	var c *cluster.Cluster
	var err error
	if *config.Cluster.NodeID > 0 {
		exitIfInvalidClusterParams(*config.Cluster.NodeID, *config.Cluster.NodePort, *config.Cluster.Remotes)
		logger.Info("Starting in cluster-mode")
		c, err = cluster.New(&cluster.Config{
			ID:      *config.Cluster.NodeID,
			Port:    *config.Cluster.NodePort,
			Remotes: *config.Cluster.Remotes,
		})
		if err != nil {
			logger.WithField("err", err).Fatal("Service could not be started (cluster)")
		}
	} else {
		logger.Info("Starting in standalone-mode")
	}

	router := server.NewRouter(accessManager, messageStore, kvStore, c)
	webserver := webserver.New(*config.HttpListen)

	service := server.NewService(router, webserver).
		HealthEndpoint(*config.HealthEndpoint).
		MetricsEndpoint(*config.Metrics.Endpoint)

	service.RegisterModules(CreateModules(router)...)

	if err = service.Start(); err != nil {
		if err = service.Stop(); err != nil {
			logger.WithField("err", err).Error("Error when stopping service after Start() failed")
		}
		logger.WithField("err", err).Fatal("Service could not be started")
	}

	return service
}

func exitIfInvalidClusterParams(nodeID int, nodePort int, remotes []*net.TCPAddr) {
	if (nodeID <= 0 && len(remotes) > 0) || (nodePort <= 0) {
		errorMessage := "Could not start in cluster-mode: invalid/incomplete parameters"
		logger.WithFields(log.Fields{
			"nodeID":          nodeID,
			"nodePort":        nodePort,
			"numberOfRemotes": len(remotes),
		}).Fatal(errorMessage)
	}
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
