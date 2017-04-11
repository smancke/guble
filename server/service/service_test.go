package service

import (
	"github.com/cosminrentea/gobbler/server/kvstore"
	"github.com/cosminrentea/gobbler/server/store"
	"github.com/cosminrentea/gobbler/server/store/dummystore"
	"github.com/cosminrentea/gobbler/server/webserver"
	"github.com/cosminrentea/gobbler/testutil"

	"github.com/stretchr/testify/assert"

	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

func TestStartingOfModules(t *testing.T) {
	defer testutil.ResetDefaultRegistryHealthCheck()

	a := assert.New(t)
	var p interface{}
	func() {
		defer func() {
			p = recover()
		}()

		service, _, _, _ := aMockedServiceWithMockedRouterStandalone()
		service.RegisterModules(0, 0, &testStartable{})
		a.Equal(3, len(service.ModulesSortedByStartOrder()))
		service.Start()
	}()
	a.NotNil(p)
}

func TestStoppingOfModules(t *testing.T) {
	defer testutil.ResetDefaultRegistryHealthCheck()

	var p interface{}
	func() {
		defer func() {
			p = recover()
		}()

		service, _, _, _ := aMockedServiceWithMockedRouterStandalone()
		service.RegisterModules(0, 0, &testStopable{})
		service.Stop()
	}()
	assert.NotNil(t, p)
}

func TestEndpointRegisterAndServing(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	defer testutil.ResetDefaultRegistryHealthCheck()
	a := assert.New(t)

	// given:
	service, _, _, _ := aMockedServiceWithMockedRouterStandalone()

	// when I register an endpoint at path /foo
	service.RegisterModules(0, 0, &testEndpoint{})
	a.Equal(3, len(service.ModulesSortedByStartOrder()))
	service.Start()
	defer service.Stop()
	time.Sleep(time.Millisecond * 10)

	// then I can call the handler
	url := fmt.Sprintf("http://%s/foo", service.WebServer().GetAddr())
	result, err := http.Get(url)
	a.NoError(err)
	body := make([]byte, 3)
	result.Body.Read(body)
	a.Equal("bar", string(body))
}

func TestHealthUp(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	defer testutil.ResetDefaultRegistryHealthCheck()
	a := assert.New(t)

	// given:
	service, _, _, _ := aMockedServiceWithMockedRouterStandalone()
	service = service.HealthEndpoint("/health_url")
	a.Equal(2, len(service.ModulesSortedByStartOrder()))

	// when starting the service
	defer service.Stop()
	service.Start()
	time.Sleep(time.Millisecond * 10)

	// and when I call the health URL
	url := fmt.Sprintf("http://%s/health_url", service.WebServer().GetAddr())
	result, err := http.Get(url)

	// then I get status 200 and JSON: {}
	a.NoError(err)
	body, err := ioutil.ReadAll(result.Body)
	a.NoError(err)
	a.Equal("{}", string(body))
}

func TestHealthDown(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	defer testutil.ResetDefaultRegistryHealthCheck()
	a := assert.New(t)

	// given:
	service, _, _, _ := aMockedServiceWithMockedRouterStandalone()
	service = service.HealthEndpoint("/health_url")
	mockChecker := NewMockChecker(ctrl)
	mockChecker.EXPECT().Check().Return(errors.New("sick")).AnyTimes()

	// when starting the service with a short frequency
	defer service.Stop()
	service.healthFrequency = time.Millisecond * 3
	service.RegisterModules(0, 0, mockChecker)
	a.Equal(3, len(service.ModulesSortedByStartOrder()))

	service.Start()
	time.Sleep(time.Millisecond * 10)

	// and when I can call the health URL
	url := fmt.Sprintf("http://%s/health_url", service.WebServer().GetAddr())
	result, err := http.Get(url)
	// then I receive status 503 and a JSON error message
	a.NoError(err)
	a.Equal(503, result.StatusCode)
	body, err := ioutil.ReadAll(result.Body)
	a.NoError(err)
	a.Equal("{\"*service.MockChecker\":\"sick\"}", string(body))
}

func TestMetricsEnabled(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	defer testutil.ResetDefaultRegistryHealthCheck()
	a := assert.New(t)

	// given:
	service, _, _, _ := aMockedServiceWithMockedRouterStandalone()
	service = service.MetricsEndpoint("/metrics_url")
	a.Equal(2, len(service.ModulesSortedByStartOrder()))

	// when starting the service
	defer service.Stop()
	service.Start()
	time.Sleep(time.Millisecond * 10)

	// and when I call the health URL
	url := fmt.Sprintf("http://%s/metrics_url", service.WebServer().GetAddr())
	result, err := http.Get(url)

	// then I get status 200 and a non-empty text
	a.NoError(err)
	body, err := ioutil.ReadAll(result.Body)
	a.NoError(err)
	a.True(len(body) > 0)
}

func TestPrometheusMetricsEnabled(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	defer testutil.ResetDefaultRegistryHealthCheck()
	a := assert.New(t)

	// given:
	service, _, _, _ := aMockedServiceWithMockedRouterStandalone()
	service = service.PrometheusEndpoint("/metrics_url")
	a.Equal(2, len(service.ModulesSortedByStartOrder()))

	// when starting the service
	defer service.Stop()
	service.Start()
	time.Sleep(time.Millisecond * 10)

	// and when I call the health URL
	url := fmt.Sprintf("http://%s/metrics_url", service.WebServer().GetAddr())
	result, err := http.Get(url)

	// then I get status 200 and some non-empty text
	a.NoError(err)
	body, err := ioutil.ReadAll(result.Body)
	a.NoError(err)
	a.True(len(body) > 0)
}

func aMockedServiceWithMockedRouterStandalone() (*Service, kvstore.KVStore, store.MessageStore, *MockRouter) {
	kvStore := kvstore.NewMemoryKVStore()
	messageStore := dummystore.New(kvStore)
	routerMock := NewMockRouter(testutil.MockCtrl)
	routerMock.EXPECT().Cluster().Return(nil).MaxTimes(2)
	service := New(routerMock, webserver.New("localhost:0"))
	return service, kvStore, messageStore, routerMock
}

type testEndpoint struct {
}

func (*testEndpoint) GetPrefix() string {
	return "/foo"
}

func (*testEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "bar")
	return
}

type testStartable struct {
}

func (*testStartable) Start() error {
	panic(fmt.Errorf("In a panic when I should start"))
}

type testStopable struct {
}

func (*testStopable) Stop() error {
	panic(fmt.Errorf("In a panic when I should stop"))
}
