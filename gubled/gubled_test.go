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
