package gubled

import (
	"github.com/smancke/guble/gcm"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/server/rest"
	"github.com/smancke/guble/server/websocket"
	"github.com/smancke/guble/store"

	"fmt"
	"github.com/alexflint/go-arg"
	"github.com/caarlos0/env"
	"github.com/smancke/guble/server/auth"
	"os"
	"os/signal"
	"path"
	"syscall"
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
}

var ValidateStoragePath = func(args Args) error {
	if args.KVBackend == "file" || args.MSBackend == "file" {
		testfile := path.Join(args.StoragePath, "write-test-file")
		f, err := os.Create(testfile)
		if err != nil {
			protocol.ErrWithoutTrace("Storage path not present/writeable %q: %v", args.StoragePath, err)
			if args.StoragePath == "/var/lib/guble" {
				protocol.ErrWithoutTrace("Use --storage-path=<path> to override the default location, or create the directory with RW rights.")
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
			panic(err)
		}
		return db
	default:
		panic(fmt.Errorf("unknown key value backend: %q", args.KVBackend))
	}
}

var CreateMessageStore = func(args Args) store.MessageStore {
	switch args.MSBackend {
	case "none", "":
		return store.NewDummyMessageStore(store.NewMemoryKVStore())
	case "file":
		protocol.Info("using FileMessageStore in directory: %q", args.StoragePath)
		return store.NewFileMessageStore(args.StoragePath)
	default:
		panic(fmt.Errorf("unknown message store backend: %q", args.MSBackend))
	}
}

var CreateModules = func(router server.Router, args Args) []interface{} {
	modules := make([]interface{}, 0, 2)

	if wsHandler, err := websocket.NewWSHandler(router, "/stream/"); err != nil {
		protocol.Err("Error loading WSHandler module: %s", err)
	} else {
		modules = append(modules, wsHandler)
	}

	modules = append(modules, rest.NewRestMessageAPI(router, "/api/"))

	if args.GcmEnable {
		if args.GcmApiKey == "" {
			panic("gcm api key has to be provided, if gcm is enabled")
		}

		protocol.Info("google cloud messaging: enabled")
		if gcm, err := gcm.NewGCMConnector(router, "/gcm/", args.GcmApiKey); err != nil {
			protocol.Err("Error loading GCMConnector: ", err)
		} else {
			modules = append(modules, gcm)
		}
	} else {
		protocol.Info("google cloud messaging: disabled")
	}

	return modules
}

func Main() {
	defer func() {
		if p := recover(); p != nil {
			protocol.Err("%v", p)
			os.Exit(1)
		}
	}()

	args := loadArgs()
	if args.LogInfo {
		protocol.LogLevel = protocol.LEVEL_INFO
	}
	if args.LogDebug {
		protocol.LogLevel = protocol.LEVEL_DEBUG
	}

	if err := ValidateStoragePath(args); err != nil {
		os.Exit(1)
	}

	service := StartupService(args)

	waitForTermination(func() {
		err := service.Stop()
		if err != nil {
			protocol.Err("Service: ", err)
		}
	})
}

func StartupService(args Args) *server.Service {
	accessManager := auth.NewAllowAllAccessManager(true)
	messageStore := CreateMessageStore(args)
	kvStore := CreateKVStore(args)

	router := server.NewRouter(accessManager, messageStore, kvStore)

	service := server.NewService(args.Listen, router)

	for _, module := range CreateModules(router, args) {
		service.Register(module)
	}

	if err := service.Start(); err != nil {
		protocol.Err(err.Error())
		if err := service.Stop(); err != nil {
			protocol.Err(err.Error())
		}
		os.Exit(1)
	}

	return service
}

func loadArgs() Args {
	args := Args{
		Listen:      ":8080",
		KVBackend:   "file",
		MSBackend:   "file",
		StoragePath: "/var/lib/guble",
	}

	env.Parse(&args)
	arg.MustParse(&args)
	return args
}

func waitForTermination(callback func()) {
	sigc := make(chan os.Signal)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	protocol.Info("Got signal '%v' .. exiting gracefully now", <-sigc)
	callback()
	protocol.Info("exit now")
	os.Exit(0)
}
