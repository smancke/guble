package gubled

import (
	"github.com/smancke/guble/store"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"

	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

func TestValidateStoragePath(t *testing.T) {
	a := assert.New(t)

	valid := os.TempDir()
	invalid := os.TempDir() + "/non-existing-directory-for-guble-test"

	a.NoError(ValidateStoragePath(Args{StoragePath: valid, MSBackend: "file"}))
	a.NoError(ValidateStoragePath(Args{StoragePath: invalid}))

	a.Error(ValidateStoragePath(Args{StoragePath: invalid, MSBackend: "file"}))
	a.Error(ValidateStoragePath(Args{StoragePath: invalid, KVBackend: "file"}))
}

func TestCreateKVStoreBackend(t *testing.T) {
	a := assert.New(t)

	memory := CreateKVStore(Args{KVBackend: "memory"})
	a.Equal("*store.MemoryKVStore", reflect.TypeOf(memory).String())

	dir, _ := ioutil.TempDir("", "guble_test")
	defer os.RemoveAll(dir)
	sqlite := CreateKVStore(Args{KVBackend: "file", StoragePath: dir})
	a.Equal("*store.SqliteKVStore", reflect.TypeOf(sqlite).String())
}

func TestParsingOfEnviromentVariables(t *testing.T) {
	a := assert.New(t)

	originalArgs := os.Args
	os.Args = []string{os.Args[0]}
	defer func() { os.Args = originalArgs }()

	// given: some environment variables
	os.Setenv("GUBLE_LISTEN", "listen")
	defer os.Unsetenv("GUBLE_LISTEN")

	os.Setenv("GUBLE_LOG_INFO", "true")
	defer os.Unsetenv("GUBLE_LOG_INFO")

	os.Setenv("GUBLE_LOG_DEBUG", "true")
	defer os.Unsetenv("GUBLE_LOG_DEBUG")

	os.Setenv("GUBLE_KV_BACKEND", "kv-backend")
	defer os.Unsetenv("GUBLE_KV_BACKEND")

	os.Setenv("GUBLE_STORAGE_PATH", "storage-path")
	defer os.Unsetenv("GUBLE_STORAGE_PATH")

	os.Setenv("GUBLE_MS_BACKEND", "ms-backend")
	defer os.Unsetenv("GUBLE_MS_BACKEND")

	os.Setenv("GUBLE_GCM_API_KEY", "gcm-api-key")
	defer os.Unsetenv("GUBLE_GCM_API_KEY")

	os.Setenv("GUBLE_GCM_ENABLE", "true")
	defer os.Unsetenv("GUBLE_GCM_ENABLE")

	os.Setenv("GUBLE_GCM_WORKERS", "4")
	defer os.Unsetenv("GUBLE_GCM_WORKERS")

	os.Setenv("GUBLE_NODE_ID", "1")
	defer os.Unsetenv("GUBLE_NODE_ID")

	// when we parse the arguments
	args := loadArgs()

	// then the args parameters are set
	assertArguments(a, args, false)
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
		"--storage-path", "storage-path",
		"--ms-backend", "ms-backend",
		"--gcm-api-key", "gcm-api-key",
		"--node-id", "1",
		"--gcm-enable",
		"--gcm-workers", "4",
		"http://example.com:8080", "https://example.com:8908"}

	// when we parse the arguments
	args := loadArgs()

	// the the arg parameters are set
	assertArguments(a, args, true)
}

func assertArguments(a *assert.Assertions, args Args, useNodeUrls bool) {
	a.Equal("listen", args.Listen)
	a.Equal(true, args.LogInfo)
	a.Equal(true, args.LogDebug)
	a.Equal("kv-backend", args.KVBackend)
	a.Equal("storage-path", args.StoragePath)
	a.Equal("ms-backend", args.MSBackend)
	a.Equal("gcm-api-key", args.GcmApiKey)
	a.Equal(4, args.GcmWorkers)
	a.Equal(1, args.NodeId)
	a.Equal(true, args.GcmEnable)

	if useNodeUrls == true {
		a.Equal(createTestSlice("http://example.com:8080", "https://example.com:8908"), args.Remotes)
	}
}

func createTestSlice(s1, s2 string) []string {
	sliceString := make([]string, 0)
	sliceString = append(sliceString, s1)
	sliceString = append(sliceString, s2)
	return sliceString
}

func TestValidateUrl(t *testing.T) {
	a := assert.New(t)

	originalArgs := os.Args

	defer func() {
		os.Args = originalArgs
	}()

	// given: a command line
	os.Args = []string{os.Args[0],
		"--listen", "listen",
		"--log-info",
		"--log-debug",
		"--kv-backend", "kv-backend",
		"--storage-path", "storage-path",
		"--ms-backend", "ms-backend",
		"--gcm-api-key", "gcm-api-key",
		"--node-id", "1",
		"--gcm-enable",
		"--gcm-workers", "4",
		"http://example.com:8080", "https://example.com:8908"}

	// when we parse the arguments
	args := loadArgs()
	parsedHosts := validateRemoteHostsWithPorts(args.Remotes)

	a.Equal(parsedHosts, createTestSlice("example.com:8080", "example.com:8908"))
}

func TestArgDefaultValues(t *testing.T) {
	a := assert.New(t)

	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// given: a command line
	os.Args = []string{os.Args[0]}

	// when we parse the arguments
	args := loadArgs()

	// the we have correct defaults set
	a.Equal(":8080", args.Listen)
	a.Equal(false, args.LogInfo)
	a.Equal(false, args.LogDebug)
	a.Equal("file", args.KVBackend)
	a.Equal("/var/lib/guble", args.StoragePath)
	a.Equal("file", args.MSBackend)
	a.Equal("", args.GcmApiKey)
	a.Equal(false, args.GcmEnable)
}

func TestGcmOnlyStartedIfEnabled(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	routerMock, _, _ := initRouterMock()
	routerMock.EXPECT().KVStore().Return(store.NewMemoryKVStore(), nil)

	a.True(containsGcmModule(CreateModules(routerMock, Args{GcmEnable: true, GcmApiKey: "xyz"})))
	a.False(containsGcmModule(CreateModules(routerMock, Args{GcmEnable: false})))
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
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	defer func() {
		if r := recover(); r == nil {
			t.Log("expect panic, because the gcm api key was not supplied")
			t.Fail()
		}
	}()

	routerMock, _, _ := initRouterMock()

	CreateModules(routerMock, Args{GcmEnable: true})
}

func TestCreateStoreBackendPanicInvalidBackend(t *testing.T) {
	a := assert.New(t)

	var p interface{}
	func() {
		defer func() {
			p = recover()
		}()

		CreateKVStore(Args{KVBackend: "foo bar"})
	}()
	a.NotNil(p)
}

func initRouterMock() (*MockRouter, *MockAccessManager, *MockMessageStore) {
	routerMock := NewMockRouter(testutil.MockCtrl)
	amMock := NewMockAccessManager(testutil.MockCtrl)
	msMock := NewMockMessageStore(testutil.MockCtrl)

	routerMock.EXPECT().AccessManager().Return(amMock, nil).AnyTimes()
	routerMock.EXPECT().MessageStore().Return(msMock, nil).AnyTimes()

	return routerMock, amMock, msMock
}
