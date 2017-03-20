package server

import (
	log "github.com/Sirupsen/logrus"

	"github.com/cosminrentea/gobbler/logformatter"
	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/apns"
	"github.com/cosminrentea/gobbler/server/auth"
	"github.com/cosminrentea/gobbler/server/cluster"
	"github.com/cosminrentea/gobbler/server/fcm"
	"github.com/cosminrentea/gobbler/server/kvstore"
	"github.com/cosminrentea/gobbler/server/metrics"
	"github.com/cosminrentea/gobbler/server/rest"
	"github.com/cosminrentea/gobbler/server/router"
	"github.com/cosminrentea/gobbler/server/service"
	"github.com/cosminrentea/gobbler/server/sms"
	"github.com/cosminrentea/gobbler/server/store"
	"github.com/cosminrentea/gobbler/server/store/dummystore"
	"github.com/cosminrentea/gobbler/server/store/filestore"
	"github.com/cosminrentea/gobbler/server/webserver"
	"github.com/cosminrentea/gobbler/server/websocket"

	"fmt"
	"net"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"syscall"

	"github.com/Bogh/gcm"
	"github.com/pkg/profile"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	fileOption = "file"
)

var AfterMessageDelivery = func(m *protocol.Message) {
	logger.WithField("message", m).Debug("message delivered")
}

// ValidateStoragePath validates the guble configuration with regard to the storagePath
// (which can be used by MessageStore and/or KVStore implementations).
var ValidateStoragePath = func() error {
	if *Config.KVS == fileOption || *Config.MS == fileOption {
		testfile := path.Join(*Config.StoragePath, "write-test-file")
		f, err := os.Create(testfile)
		if err != nil {
			logger.WithError(err).WithField("storagePath", *Config.StoragePath).Error("Storage path not present/writeable.")
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
	switch *Config.KVS {
	case "memory":
		return kvstore.NewMemoryKVStore()
	case "file":
		db := kvstore.NewSqliteKVStore(path.Join(*Config.StoragePath, "kv-store.db"), true)
		if err := db.Open(); err != nil {
			logger.WithError(err).Panic("Could not open sqlite database connection")
		}
		return db
	case "postgres":
		db := kvstore.NewPostgresKVStore(kvstore.PostgresConfig{
			ConnParams: map[string]string{
				"host":     *Config.Postgres.Host,
				"port":     strconv.Itoa(*Config.Postgres.Port),
				"user":     *Config.Postgres.User,
				"password": *Config.Postgres.Password,
				"dbname":   *Config.Postgres.DbName,
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
		panic(fmt.Errorf("Unknown key-value backend: %q", *Config.KVS))
	}
}

// CreateMessageStore is a func which returns a store.MessageStore implementation
// (currently, based on guble configuration).
var CreateMessageStore = func() store.MessageStore {
	switch *Config.MS {
	case "none", "memory", "":
		return dummystore.New(kvstore.NewMemoryKVStore())
	case "file":
		logger.WithField("storagePath", *Config.StoragePath).Info("Using FileMessageStore in directory")
		return filestore.New(*Config.StoragePath)
	default:
		panic(fmt.Errorf("Unknown message-store backend: %q", *Config.MS))
	}
}

// CreateModules is a func which returns a slice of modules which should be used by the service
// (currently, based on guble configuration);
// see package `service` for terminological details.
var CreateModules = func(router router.Router) (modules []interface{}) {

	modules = append(modules, rest.NewRestMessageAPI(router, "/api/"))

	if *Config.WS.Enabled {
		if wsHandler, err := websocket.NewWSHandler(router, *Config.WS.Prefix); err != nil {
			logger.WithError(err).Error("Error loading WSHandler module")
		} else {
			modules = append(modules, wsHandler)
		}
	}

	if *Config.FCM.Enabled {
		logger.Info("Firebase Cloud Messaging: enabled")
		if *Config.FCM.APIKey == "" {
			logger.Panic("The API Key has to be provided when Firebase Cloud Messaging is enabled")
		}
		Config.FCM.AfterMessageDelivery = AfterMessageDelivery
		*Config.FCM.IntervalMetrics = true
		if Config.FCM.Endpoint != nil {
			gcm.GcmSendEndpoint = *Config.FCM.Endpoint
		}
		sender := fcm.NewSender(*Config.FCM.APIKey)
		if fcmConn, err := fcm.New(router, sender, Config.FCM); err != nil {
			logger.WithError(err).Error("Error creating FCM connector")
		} else {
			modules = append(modules, fcmConn)
		}
	} else {
		logger.Info("Firebase Cloud Messaging: disabled")
	}

	if *Config.APNS.Enabled {
		if *Config.APNS.Production {
			logger.Info("APNS: enabled in production mode")
		} else {
			logger.Info("APNS: enabled in development mode")
		}
		logger.Info("APNS: enabled")
		if *Config.APNS.CertificateFileName == "" && Config.APNS.CertificateBytes == nil {
			logger.Panic("The certificate (as filename or bytes) has to be provided when APNS is enabled")
		}
		if *Config.APNS.CertificatePassword == "" {
			logger.Panic("A non-empty password has to be provided when APNS is enabled")
		}
		if *Config.APNS.AppTopic == "" {
			logger.Panic("The Mobile App Topic (usually the bundle-id) has to be provided when APNS is enabled")
		}
		apnsSender, err := apns.NewSender(Config.APNS)
		if err != nil {
			logger.Panic("APNS Sender could not be created")
		}
		*Config.APNS.IntervalMetrics = true
		if apnsConn, err := apns.New(router, apnsSender, Config.APNS); err != nil {
			logger.WithError(err).Error("Error creating APNS connector")
		} else {
			modules = append(modules, apnsConn)
		}
	} else {
		logger.Info("APNS: disabled")
	}

	if *Config.SMS.Enabled {
		logger.Info("Nexmo SMS: enabled")
		if *Config.SMS.APIKey == "" || *Config.SMS.APISecret == "" {
			logger.Panic("The API Key has to be provided when NEXMO SMS connector is enabled")
		}
		nexmoSender, err := sms.NewNexmoSender(*Config.SMS.APIKey, *Config.SMS.APISecret)
		if err != nil {
			logger.WithError(err).Error("Error creating Nexmo Sender")
		}
		smsConn, err := sms.New(router, nexmoSender, Config.SMS)
		if err != nil {
			logger.WithError(err).Error("Error creating Nexmo Sender")
		} else {
			modules = append(modules, smsConn)
		}
	} else {
		logger.Info("SMS: disabled")
	}

	return
}

// Main is the entry-point of the guble server.
func Main() {
	defer func() {
		if p := recover(); p != nil {
			logger.Fatal("Fatal error in gubled after recover")
		}
	}()

	parseConfig()

	if !terminal.IsTerminal(int(os.Stdout.Fd())) {
		log.SetFormatter(&logformatter.LogstashFormatter{Env: *Config.EnvName})
	}

	level, err := log.ParseLevel(*Config.Log)
	if err != nil {
		logger.WithError(err).Fatal("Invalid log level")
	}
	log.SetLevel(level)

	switch *Config.Profile {
	case cpuProfile:
		logger.Info("starting to profile cpu")
		defer profile.Start(profile.CPUProfile).Stop()
	case memProfile:
		logger.Info("starting to profile memory")
		defer profile.Start(profile.MemProfile).Stop()
	case blockProfile:
		logger.Info("starting to profile blocking/contention")
		defer profile.Start(profile.BlockProfile).Stop()
	default:
		logger.Debug("no profiling was started")
	}

	if err := ValidateStoragePath(); err != nil {
		logger.Fatal("Fatal error in gubled in validation of storage path")
	}

	srv := StartService()
	if srv == nil {
		logger.Fatal("exiting because of unrecoverable error(s) when starting the service")
	}

	waitForTermination(func() {
		err := srv.Stop()
		if err != nil {
			logger.WithField("error", err.Error()).Error("errors occurred while stopping service")
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

	if *Config.Cluster.NodeID > 0 {
		exitIfInvalidClusterParams(*Config.Cluster.NodeID, *Config.Cluster.NodePort, *Config.Cluster.Remotes)
		logger.Info("Starting in cluster-mode")
		cl, err = cluster.New(&cluster.Config{
			ID:      *Config.Cluster.NodeID,
			Port:    *Config.Cluster.NodePort,
			Remotes: *Config.Cluster.Remotes,
		})
		if err != nil {
			logger.WithField("err", err).Fatal("Module could not be started (cluster)")
		}
	} else {
		logger.Info("Starting in standalone-mode")
	}

	r := router.New(accessManager, messageStore, kvStore, cl)
	websrv := webserver.New(*Config.HttpListen)

	srv := service.New(r, websrv).
		HealthEndpoint(*Config.HealthEndpoint).
		MetricsEndpoint(*Config.MetricsEndpoint)

	srv.RegisterModules(0, 6, kvStore, messageStore)
	srv.RegisterModules(4, 3, CreateModules(r)...)

	if err = srv.Start(); err != nil {
		logger.WithField("error", err.Error()).Error("errors occurred while starting service")
		if err = srv.Stop(); err != nil {
			logger.WithField("error", err.Error()).Error("errors occurred when stopping service after it failed to start")
		}
		return nil
	}

	return srv
}

func exitIfInvalidClusterParams(nodeID uint8, nodePort int, remotes []*net.TCPAddr) {
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
	sig := <-signalC
	logger.Infof("Got signal '%v' .. exiting gracefully now", sig)
	callback()
	metrics.LogOnDebugLevel()
	logger.Info("Exit gracefully now")
	os.Exit(0)
}
