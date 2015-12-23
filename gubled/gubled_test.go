package gubled

import (
	"github.com/stretchr/testify/assert"

	"os"
	"reflect"
	"testing"
)

func TestCreateStoreBackend(t *testing.T) {
	a := assert.New(t)

	memory := CreateStoreBackend(Args{KVBackend: "memory"})
	a.Equal("*store.MemoryKVStore", reflect.TypeOf(memory).String())

	sqlite := CreateStoreBackend(Args{KVBackend: "sqlite"})
	a.Equal("*store.SqliteKVStore", reflect.TypeOf(sqlite).String())
}

func TestParsingOfEnviromentVariables(t *testing.T) {
	a := assert.New(t)

	originalArgs := os.Args
	os.Args = []string{os.Args[0]}
	defer func() { os.Args = originalArgs }()

	// given: some environment variables
	os.Setenv("GUBLE_LISTEN", "listen")
	os.Setenv("GUBLE_LOG_INFO", "true")
	os.Setenv("GUBLE_LOG_DEBUG", "true")
	os.Setenv("GUBLE_KV_BACKEND", "kv-backend")
	os.Setenv("GUBLE_KV_SQLITE_PATH", "kv-sqlite-path")
	os.Setenv("GUBLE_KV_SQLITE_NO_SYNC", "true")
	os.Setenv("GUBLE_GCM_API_KEY", "gcm-api-key")
	os.Setenv("GUBLE_GCM_ENABLE", "true")

	// when we parse the arguments
	args := loadArgs()

	// the the arg parameters are set
	assertArguments(a, args)
}

func TestParsingArgs(t *testing.T) {
	a := assert.New(t)

	originalArgs := os.Args

	defer func() { os.Args = originalArgs }()

	// given: a command line
	os.Args = []string{os.Args[0],
		"--listen", "listen",
		"--log-info",
		"--log-debug",
		"--kv-backend", "kv-backend",
		"--kv-sqlite-path", "kv-sqlite-path",
		"--kv-sqlite-no-sync",
		"--gcm-api-key", "gcm-api-key",
		"--gcm-enable"}

	// when we parse the arguments
	args := loadArgs()

	// the the arg parameters are set
	assertArguments(a, args)
}

func assertArguments(a *assert.Assertions, args Args) {
	a.Equal("listen", args.Listen)
	a.Equal(true, args.LogInfo)
	a.Equal(true, args.LogDebug)
	a.Equal("kv-backend", args.KVBackend)
	a.Equal("kv-sqlite-path", args.KVSqlitePath)
	a.Equal(true, args.KVSqliteNoSync)
	a.Equal("gcm-api-key", args.GcmApiKey)
	a.Equal(true, args.GcmEnable)
}

func TestGcmOnlyStartedIfEnabled(t *testing.T) {
	a := assert.New(t)

	a.True(containsGcmModule(CreateModules(Args{GcmEnable: true, GcmApiKey: "xyz"})))
	a.False(containsGcmModule(CreateModules(Args{GcmEnable: false})))
}

func containsGcmModule(modules []interface{}) bool {
	for _, module := range modules {
		if reflect.TypeOf(module).String() == "*gcm.GCMConnector" {
			return true
		}
	}
	return false
}

func TestPanicOnMissingGcmApiKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Log("expect panic, because the gcm api key was not supplied")
			t.Fail()
		}
	}()

	CreateModules(Args{GcmEnable: true})
}

func TestCreateStoreBackendPanicInvalidBackend(t *testing.T) {
	a := assert.New(t)

	var p interface{}
	func() {
		defer func() {
			p = recover()
		}()

		CreateStoreBackend(Args{KVBackend: "foo bar"})
	}()
	a.NotNil(p)
}
