package server

import (
	log "github.com/Sirupsen/logrus"

	"github.com/smancke/guble/logformatter"
	"github.com/smancke/guble/server/auth"
	"github.com/smancke/guble/server/cluster"
	"github.com/smancke/guble/server/gcm"
	"github.com/smancke/guble/server/kvstore"
	"github.com/smancke/guble/server/metrics"
	"github.com/smancke/guble/server/rest"
	"github.com/smancke/guble/server/service"
	"github.com/smancke/guble/server/store"
	"github.com/smancke/guble/server/store/dummystore"
	"github.com/smancke/guble/server/store/filestore"
	"github.com/smancke/guble/server/webserver"
	"github.com/smancke/guble/server/websocket"

	"fmt"
	"net"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"syscall"

	"github.com/smancke/guble/server/router"
)

const (
	fileOption = "file"
)

// ValidateStoragePath validates the guble configuration with regard to the storagePath
// (which can be used by MessageStore and/or KVStore implementations).
var ValidateStoragePath = func() error {
	if *config.KVS == fileOption || *config.MS == fileOption {
		testfile := path.Join(*config.StoragePath, "write-test-file")
		f, err := os.Create(testfile)
		if err != nil {
			logger.WithError(err).WithField("storagePath", *config.StoragePath).Error("Storage path not present/writeable.")
			return err
		}
		f.Close()
		os.Remove(testfile)
	}
	return nil
}

// CreateAccessManager is a func which returns a auth.AccessManager implementation
// (currently: AllowAllAccessManager).
var CreateAccessManager = func() auth.AccessManager {
	return auth.NewAllowAllAccessManager(true)
}

// CreateKVStore is a func which returns a kvstore.KVStore implementation
// (currently, based on guble configuration).
var CreateKVStore = func() kvstore.KVStore {
	switch *config.KVS {
	case "memory":
		return kvstore.NewMemoryKVStore()
	case "file":
		db := kvstore.NewSqliteKVStore(path.Join(*config.StoragePath, "kv-store.db"), true)
		if err := db.Open(); err != nil {
			logger.WithError(err).Panic("Could not open sqlite database connection")
		}
		return db
	case "postgres":
		db := kvstore.NewPostgresKVStore(kvstore.PostgresConfig{
			ConnParams: map[string]string{
				"host":     *config.Postgres.Host,
				"port":     strconv.Itoa(*config.Postgres.Port),
				"user":     *config.Postgres.User,
				"password": *config.Postgres.Password,
				"dbname":   *config.Postgres.DbName,
				"sslmode":  "disable",
			},
			MaxIdleConns: 1,
			MaxOpenConns: runtime.GOMAXPROCS(0),
		})
		if err := db.Open(); err != nil {
			logger.WithError(err).Panic("Could not open postgres database connection")
		}
		return db
	default:
		panic(fmt.Errorf("Unknown key-value backend: %q", *config.KVS))
	}
}

// CreateMessageStore is a func which returns a store.MessageStore implementation
// (currently, based on guble configuration).
var CreateMessageStore = func() store.MessageStore {
	switch *config.MS {
	case "none", "":
		return dummystore.New(kvstore.NewMemoryKVStore())
	case "file":
		logger.WithField("storagePath", *config.StoragePath).Info("Using FileMessageStore in directory")
		return filestore.New(*config.StoragePath)
	default:
		panic(fmt.Errorf("Unknown message-store backend: %q", *config.MS))
	}
}

// CreateModules is a func which returns a slice of modules which should be used by the service
// (currently, based on guble configuration);
// see package `service` for terminological details.
var CreateModules = func(router router.Router) []interface{} {
	var modules []interface{}

	if wsHandler, err := websocket.NewWSHandler(router, "/stream/"); err != nil {
		logger.WithError(err).Error("Error loading WSHandler module:")
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

		if g, err := gcm.New(
			router,
			"/gcm/",
			*config.GCM.APIKey,
			*config.GCM.Workers,
			*config.GCM.Endpoint); err != nil {
			logger.WithError(err).Error("Error creating GCMConnector")

		} else {
			modules = append(modules, g)
		}
	} else {
		logger.Info("Google cloud messaging: disabled")
	}

	return modules
}

// Main is the entry-point of the guble server.
func Main() {
	parseConfig()

	log.SetFormatter(&logformatter.LogstashFormatter{Env: *config.EnvName})
	defer func() {
		if p := recover(); p != nil {
			logger.Fatal("Fatal error in gubled after recover")
		}
	}()

	// set log level
	level, err := log.ParseLevel(*config.Log)
	if err != nil {
		logger.WithError(err).Fatal("Invalid log level")
	}
	log.SetLevel(level)

	if err := ValidateStoragePath(); err != nil {
		logger.Fatal("Fatal error in gubled in validation of storage path")
	}

	srv := StartService()

	waitForTermination(func() {
		err := srv.Stop()
		if err != nil {
			logger.WithError(err).Error("Error when stopping service")
		}
	})
}

// StartService starts a server.Service after first creating the router (and its dependencies), the webserver.
func StartService() *service.Service {
	//TODO StartService could return an error in case it fails to start

	accessManager := CreateAccessManager()
	messageStore := CreateMessageStore()
	kvStore := CreateKVStore()

	var cl *cluster.Cluster
	var err error
	if *config.Cluster.NodeID > 0 {
		exitIfInvalidClusterParams(*config.Cluster.NodeID, *config.Cluster.NodePort, *config.Cluster.Remotes)
		logger.Info("Starting in cluster-mode")
		cl, err = cluster.New(&cluster.Config{
			ID:      *config.Cluster.NodeID,
			Port:    *config.Cluster.NodePort,
			Remotes: *config.Cluster.Remotes,
		})
		if err != nil {
			logger.WithError(err).Fatal("Service could not be started (cluster)")
		}
	} else {
		logger.Info("Starting in standalone-mode")
	}

	r := router.New(accessManager, messageStore, kvStore, cl)
	websrv := webserver.New(*config.HttpListen)

	srv := service.New(r, websrv).
		HealthEndpoint(*config.HealthEndpoint).
		MetricsEndpoint(*config.MetricsEndpoint)

	srv.RegisterModules(0, 6, kvStore, messageStore)
	srv.RegisterModules(4, 3, CreateModules(r)...)

	if err = srv.Start(); err != nil {
		logger.WithError(err).Fatal("Service could not be started")
		if err = srv.Stop(); err != nil {
			logger.WithError(err).Error("Error when stopping service after Start() failed")
		}
	}

	return srv
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
