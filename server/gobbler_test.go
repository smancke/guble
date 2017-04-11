package server

import (
	"github.com/cosminrentea/gobbler/server/kvstore"

	"github.com/cosminrentea/gobbler/testutil"
	"github.com/stretchr/testify/assert"

	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"
	"github.com/cosminrentea/gobbler/server/configstring"
)

func TestValidateStoragePath(t *testing.T) {
	a := assert.New(t)

	valid := os.TempDir()
	invalid := os.TempDir() + "/non-existing-directory-for-guble-test"

	*Config.MS = "file"

	*Config.StoragePath = valid
	a.NoError(ValidateStoragePath())
	*Config.StoragePath = invalid

	a.Error(ValidateStoragePath())

	*Config.KVS = "file"
	a.Error(ValidateStoragePath())
}

func TestCreateKVStoreBackend(t *testing.T) {
	a := assert.New(t)
	*Config.KVS = "memory"
	memory := CreateKVStore()
	a.Equal("*kvstore.MemoryKVStore", reflect.TypeOf(memory).String())

	dir, _ := ioutil.TempDir("", "guble_test")
	defer os.RemoveAll(dir)

	*Config.KVS = "file"
	*Config.StoragePath = dir
	sqlite := CreateKVStore()
	a.Equal("*kvstore.SqliteKVStore", reflect.TypeOf(sqlite).String())
}

func TestFCMOnlyStartedIfEnabled(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	routerMock := initRouterMock()
	routerMock.EXPECT().KVStore().Return(kvstore.NewMemoryKVStore(), nil)

	*Config.FCM.Enabled = true
	*Config.FCM.APIKey = "xyz"
	*Config.APNS.Enabled = false
	a.True(containsFCMModule(CreateModules(routerMock)))

	*Config.FCM.Enabled = false
	a.False(containsFCMModule(CreateModules(routerMock)))
}

func containsFCMModule(modules []interface{}) bool {
	for _, module := range modules {
		if reflect.TypeOf(module).String() == "*fcm.fcm" {
			return true
		}
	}
	return false
}

func TestPanicOnMissingFCMApiKey(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()

	defer func() {
		if r := recover(); r == nil {
			t.Log("expect panic, because the gcm api key was not supplied")
			t.Fail()
		}
	}()

	routerMock := initRouterMock()
	*Config.FCM.APIKey = ""
	*Config.FCM.Enabled = true
	CreateModules(routerMock)
}

func TestCreateStoreBackendPanicInvalidBackend(t *testing.T) {
	var p interface{}
	func() {
		defer func() {
			p = recover()
		}()

		*Config.KVS = "foo bar"
		CreateKVStore()
	}()
	assert.NotNil(t, p)
}

func TestStartServiceModules(t *testing.T) {
	defer testutil.ResetDefaultRegistryHealthCheck()

	a := assert.New(t)

	// when starting a simple valid service
	*Config.KVS = "memory"
	*Config.MS = "file"
	*Config.FCM.Enabled = false
	*Config.APNS.Enabled = false
	*Config.WS.Enabled = false
	*Config.KafkaProducer.Brokers = configstring.List{}

	// using an available port for http
	testHttpPort++
	logger.WithField("port", testHttpPort).Debug("trying to use HTTP Port")
	*Config.HttpListen = fmt.Sprintf(":%d", testHttpPort)

	s := StartService()
	defer s.Stop()

	// then the number and ordering of modules should be correct
	a.Equal(5, len(s.ModulesSortedByStartOrder()))
	var moduleNames []string
	for _, iface := range s.ModulesSortedByStartOrder() {
		name := reflect.TypeOf(iface).String()
		moduleNames = append(moduleNames, name)
	}
	a.Equal("*kvstore.MemoryKVStore *filestore.FileMessageStore *router.router *webserver.WebServer *rest.RestMessageAPI",
		strings.Join(moduleNames, " "))
}

func initRouterMock() *MockRouter {
	routerMock := NewMockRouter(testutil.MockCtrl)
	routerMock.EXPECT().Cluster().Return(nil).AnyTimes()
	msMock := NewMockMessageStore(testutil.MockCtrl)

	routerMock.EXPECT().MessageStore().Return(msMock, nil).AnyTimes()

	return routerMock
}
