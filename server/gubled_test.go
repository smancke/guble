package server

import (
	"github.com/smancke/guble/server/kvstore"

	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"

	"io/ioutil"
	"os"
	"reflect"
	"strings"
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
	a.Equal("*kvstore.MemoryKVStore", reflect.TypeOf(memory).String())

	dir, _ := ioutil.TempDir("", "guble_test")
	defer os.RemoveAll(dir)

	*config.KVS = "file"
	*config.StoragePath = dir
	sqlite := CreateKVStore()
	a.Equal("*kvstore.SqliteKVStore", reflect.TypeOf(sqlite).String())
}

func TestGcmOnlyStartedIfEnabled(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	routerMock := initRouterMock()
	routerMock.EXPECT().KVStore().Return(kvstore.NewMemoryKVStore(), nil)

	*config.GCM.Enabled = true
	*config.GCM.APIKey = "xyz"
	a.True(containsGcmModule(CreateModules(routerMock)))

	*config.GCM.Enabled = false
	a.False(containsGcmModule(CreateModules(routerMock)))
}

func containsGcmModule(modules []interface{}) bool {
	for _, module := range modules {
		if reflect.TypeOf(module).String() == "*gcm.Connector" {
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

	routerMock := initRouterMock()
	*config.GCM.APIKey = ""
	*config.GCM.Enabled = true
	CreateModules(routerMock)
}

func TestCreateStoreBackendPanicInvalidBackend(t *testing.T) {
	var p interface{}
	func() {
		defer func() {
			p = recover()
		}()

		*config.KVS = "foo bar"
		CreateKVStore()
	}()
	assert.NotNil(t, p)
}

func TestStartServiceModules(t *testing.T) {
	defer testutil.ResetDefaultRegistryHealthCheck()

	a := assert.New(t)

	// when starting a simple valid service
	*config.KVS = "memory"
	*config.MS = "file"
	*config.GCM.Enabled = false
	s := StartService()

	// then the number and ordering of modules should be correct
	a.Equal(6, len(s.ModulesSortedByStartOrder()))
	var moduleNames []string
	for _, iface := range s.ModulesSortedByStartOrder() {
		name := reflect.TypeOf(iface).String()
		moduleNames = append(moduleNames, name)
	}
	a.Equal("*kvstore.MemoryKVStore *filestore.FileMessageStore *router.router *webserver.WebServer *websocket.WSHandler *rest.RestMessageAPI",
		strings.Join(moduleNames, " "))
}

func initRouterMock() *MockRouter {
	routerMock := NewMockRouter(testutil.MockCtrl)
	amMock := NewMockAccessManager(testutil.MockCtrl)
	msMock := NewMockMessageStore(testutil.MockCtrl)

	routerMock.EXPECT().AccessManager().Return(amMock, nil).AnyTimes()
	routerMock.EXPECT().MessageStore().Return(msMock, nil).AnyTimes()

	return routerMock
}
