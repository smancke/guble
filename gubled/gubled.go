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
			guble.ErrWithoutTrace("Storage path not present/writeable %q: %v", args.StoragePath, err)
			if args.StoragePath == "/var/lib/guble" {
				guble.ErrWithoutTrace("Use --storage-path=<path> to override the default location, or create the directy with RW rights.")
			}
			return err
		}
		f.Close()
		os.Remove(testfile)
	}
	return nil
}

var CreateKVStoreBackend = func(args Args) store.KVStore {
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

var CreateMessageStoreBackend = func(args Args) store.MessageStore {
	switch args.MSBackend {
	case "none", "":
		return store.NewDummyMessageStore()
	case "file":
		guble.Info("using FileMessageStore in directory: %q", args.StoragePath)
		return store.NewFileMessageStore(args.StoragePath)
	default:
		panic(fmt.Errorf("unknown message store backend: %q", args.MSBackend))
	}
}

var CreateModules = func(args Args) []interface{} {
	modules := []interface{}{
		server.NewWSHandler("/stream/"),
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

	if err := ValidateStoragePath(args); err != nil {
		os.Exit(1)
	}

	service := StartupService(args)

	waitForTermination(func() {
		err := service.Stop()
		if err != nil {
			guble.Err("Service: ", err)
		}
	})
}

func StartupService(args Args) *server.Service {
	router := server.NewPubSubRouter()
	service := server.NewService(
		args.Listen,
		CreateKVStoreBackend(args),
		CreateMessageStoreBackend(args),
		server.NewMessageEntry(router),
		router,
		server.NewAllowAllAccessManager(true))

	for _, module := range CreateModules(args) {
		service.Register(module)
	}

	if err := service.Start(); err != nil {
		guble.Err(err.Error())
		if err := service.Stop(); err != nil {
			guble.Err(err.Error())
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
	guble.Info("Got singal '%v' .. exit greacefully now", <-sigc)
	callback()
	guble.Info("exit now")
	os.Exit(0)
}
