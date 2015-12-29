package gubled

import (
	"github.com/smancke/guble/gcm"
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"

	"fmt"
	"github.com/alexflint/go-arg"
	"github.com/caarlos0/env"
	"os"
	"os/signal"
	"path"
	"syscall"
)

type Args struct {
	Listen         string `arg:"-l,help: [Host:]Port the address to listen on (:8080)" env:"GUBLE_LISTEN"`
	LogInfo        bool   `arg:"--log-info,help: Log on INFO level (false)" env:"GUBLE_LOG_INFO"`
	LogDebug       bool   `arg:"--log-debug,help: Log on DEBUG level (false)" env:"GUBLE_LOG_DEBUG"`
	KVBackend      string `arg:"--kv-backend,help: The storage backend for the key value store to use: memory|sqlite (memory)" env:"GUBLE_KV_BACKEND"`
	KVSqlitePath   string `arg:"--kv-sqlite-path,help: The path of the sqlite db for the key value store (/var/lib/guble/kv-store.db)" env:"GUBLE_KV_SQLITE_PATH"`
	KVSqliteNoSync bool   `arg:"--kv-sqlite-no-sync,help: Disable sync the key value store after every write (enabled)" env:"GUBLE_KV_SQLITE_NO_SYNC"`
	MSBackend      string `arg:"--ms-backend,help: The message storage backend (experimental): none|file (memory)" env:"GUBLE_MS_BACKEND"`
	MSPath         string `arg:"--ms-path,help: The path for message storage if 'file' is enabled (/var/lib/guble)" env:"GUBLE_MS_PATH"`
	GcmEnable      bool   `arg:"--gcm-enable: Enable the Google Cloud Messaging Connector (false)" env:"GUBLE_GCM_ENABLE"`
	GcmApiKey      string `arg:"--gcm-api-key: The Google API Key for Google Cloud Messaging" env:"GUBLE_GCM_API_KEY"`
}

var CreateKVStoreBackend = func(args Args) store.KVStore {
	switch args.KVBackend {
	case "memory":
		return store.NewMemoryKVStore()
	case "sqlite":
		db := store.NewSqliteKVStore(args.KVSqlitePath, args.KVSqliteNoSync)
		if err := db.Open(); err != nil {
			panic(err)
		}
		return db
	default:
		panic(fmt.Errorf("unknown key value backend: %q", args.KVBackend))
	}
}

var CreateMessageStoreBackend = func(args Args) store.MessageStore {
	switch args.MSBackend {
	case "none", "":
		return store.NewDummyMessageStore()
	case "file":
		guble.Info("using FileMessageStore in directory: %q", args.MSPath)
		testfile := path.Join(args.MSPath, "testfile")
		f, err := os.Create(testfile)
		if err != nil {
			panic(fmt.Errorf("directory for message store not present/writeable %q: %v", args.MSPath, err))
		}
		f.Close()
		os.Remove(testfile)

		return store.NewFileMessageStore(args.MSPath)
	default:
		panic(fmt.Errorf("unknown message store backend: %q", args.MSBackend))
	}
}

var CreateModules = func(args Args) []interface{} {
	modules := []interface{}{
		server.NewWSHandlerFactory("/stream/"),
		server.NewRestMessageApi("/api/"),
	}

	if args.GcmEnable {
		if args.GcmApiKey == "" {
			panic("gcm api key has to be provided, if gcm is enabled")
		}
		guble.Info("google cloud messaging: enabled")
		modules = append(modules, gcm.NewGCMConnector("/gcm/", args.GcmApiKey))
	} else {
		guble.Info("google cloud messaging: disabled")
	}
	return modules
}

func Main() {
	defer func() {
		if p := recover(); p != nil {
			guble.Err("%v", p)
			os.Exit(1)
		}
	}()

	args := loadArgs()
	if args.LogInfo {
		guble.LogLevel = guble.LEVEL_INFO
	}
	if args.LogDebug {
		guble.LogLevel = guble.LEVEL_DEBUG
	}
	service := StartupService(args)

	waitForTermination(func() {
		service.Stop()
	})
}

func StartupService(args Args) *server.Service {
	router := server.NewPubSubRouter().Go()
	service := server.NewService(
		args.Listen,
		CreateKVStoreBackend(args),
		CreateMessageStoreBackend(args),
		server.NewMessageEntry(router),
		router,
	)

	for _, module := range CreateModules(args) {
		service.Register(module)
	}

	service.Start()

	return service
}

func loadArgs() Args {
	args := Args{
		Listen:       ":8080",
		KVBackend:    "memory",
		KVSqlitePath: "/var/lib/guble/kv-store.db",
		MSBackend:    "none",
		MSPath:       "/var/lib/guble",
	}

	env.Parse(&args)
	arg.MustParse(&args)
	return args
}

func waitForTermination(callback func()) {
	sigc := make(chan os.Signal)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	guble.Info("Got singal '%v' .. exit greacefully now", <-sigc)
	callback()
	guble.Info("exit now")
	os.Exit(0)
}
