package gubled

import (
	"github.com/smancke/guble/gubled/config"
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

	*config.MS = "file"

	*config.StoragePath = valid
	a.NoError(ValidateStoragePath())
	*config.StoragePath = invalid

	a.Error(ValidateStoragePath())

	*config.KVS = "file"
	a.Error(ValidateStoragePath())
}

func TestCreateKVStoreBackend(t *testing.T) {
	a := assert.New(t)
	*config.KVS = "memory"
	memory := CreateKVStore()
	a.Equal("*store.MemoryKVStore", reflect.TypeOf(memory).String())

	dir, _ := ioutil.TempDir("", "guble_test")
	defer os.RemoveAll(dir)

	*config.KVS = "file"
	*config.StoragePath = dir
	sqlite := CreateKVStore()
	a.Equal("*store.SqliteKVStore", reflect.TypeOf(sqlite).String())
}

func TestGcmOnlyStartedIfEnabled(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	routerMock, _, _ := initRouterMock()
	routerMock.EXPECT().KVStore().Return(store.NewMemoryKVStore(), nil)

	*config.GCM.Enabled = true
	*config.GCM.APIKey = "xyz"
	a.True(containsGcmModule(CreateModules(routerMock)))

	*config.GCM.Enabled = false
	a.False(containsGcmModule(CreateModules(routerMock)))
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
	*config.GCM.APIKey = ""
	*config.GCM.Enabled = true
	CreateModules(routerMock)
}

func TestCreateStoreBackendPanicInvalidBackend(t *testing.T) {
	a := assert.New(t)

	var p interface{}
	func() {
		defer func() {
			p = recover()
		}()

		*config.KVS = "foo bar"
		CreateKVStore()
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
