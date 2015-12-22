package gubled

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/smancke/guble/gcm"
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"

	"fmt"
	"github.com/alexflint/go-arg"
	"github.com/caarlos0/env"
)

type Args struct {
	Listen         string `arg:"-l,help: [Host:]Port the address to listen on (:8080)" env:"GUBLE_LISTEN"`
	LogInfo        bool   `arg:"--log-info,help: Log on INFO level (false)" env:"GUBLE_LOG_INFO"`
	LogDebug       bool   `arg:"--log-debug,help: Log on DEBUG level (false)" env:"GUBLE_LOG_DEBUG"`
	KVBackend      string `arg:"--kv-backend,help: The storage backend for the key value store to use: memory|sqlite (sqlite)" env:"GUBLE_KV_BACKEND"`
	KVSqlitePath   string `arg:"--kv-sqlite-path,help: The path of the sqlite db for the key value store (/var/lib/guble/kv-store.db)" env:"GUBLE_KV_SQLITE_PATH"`
	KVSqliteNoSync bool   `arg:"--kv-sqlite-no-sync,help: Disable sync the key value store after every write (enabled)" env:"GUBLE_KV_SQLITE_NO_SYNC"`
	gcmApiKey      string `arg:"--gcm-api-key: The Google API Key for Google Cloud Messaging" env:"GUBLE_GCM_APIKEY"`
}

var CreateStoreBackend = func(args Args) store.KVStore {
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
		guble.Err("unknown key value backend: %q", args.KVBackend)
		panic(fmt.Errorf("unknown key value backend: %q", args.KVBackend))
	}
}

var CreateModules = func(args Args) []interface{} {
	return []interface{}{
		server.NewWSHandlerFactory("/stream/"),
		server.NewRestMessageApi("/api/"),
		gcm.NewGCMConnector("/gcm/", args.gcmApiKey),
	}
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
		CreateStoreBackend(args),
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
		KVBackend:    "sqlite",
		KVSqlitePath: "/var/lib/guble/kv-store.db",
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
