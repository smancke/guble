package server

import (
	"github.com/smancke/guble/server/webserver"
	"github.com/smancke/guble/store"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"

	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

func TestStopingOfModules(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	defer testutil.ResetDefaultRegistryHealthCheck()

	// given:
	service, _, _, _ := aMockedService()

	// with a registered Stopable
	stopable := NewMockStopable(ctrl)
	service.RegisterModules(stopable)

	service.Start()

	// when i stop the service, the Stop() is called
	stopable.EXPECT().Stop()
	service.Stop()
}

func TestEndpointRegisterAndServing(t *testing.T) {
	_, finish := testutil.NewMockCtrl(t)
	defer finish()
	defer testutil.ResetDefaultRegistryHealthCheck()
	a := assert.New(t)

	// given:
	service, _, _, _ := aMockedService()

	// when I register an endpoint at path /foo
	service.RegisterModules(&TestEndpoint{})
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
	service, _, _, _ := aMockedService()

	// when starting the service
	defer service.Stop()
	service.Start()
	time.Sleep(time.Millisecond * 10)

	// and when I call the health URL
	url := fmt.Sprintf("http://%s/health", service.WebServer().GetAddr())
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
	service, _, _, _ := aMockedService()
	mockChecker := NewMockChecker(ctrl)
	mockChecker.EXPECT().Check().Return(errors.New("sick")).AnyTimes()

	// when starting the service with a short frequency
	defer service.Stop()
	service.healthCheckFrequency = time.Millisecond * 3
	service.RegisterModules(mockChecker)
	service.Start()
	time.Sleep(time.Millisecond * 10)

	// and when I can call the health URL
	url := fmt.Sprintf("http://%s/health", service.WebServer().GetAddr())
	result, err := http.Get(url)
	// then I receive status 503 and a JSON error message
	a.NoError(err)
	a.Equal(503, result.StatusCode)
	body, err := ioutil.ReadAll(result.Body)
	a.NoError(err)
	a.Equal("{\"*server.MockChecker\":\"sick\"}", string(body))
}

func aMockedService() (*Service, store.KVStore, store.MessageStore, *MockRouter) {
	kvStore := store.NewMemoryKVStore()
	messageStore := store.NewDummyMessageStore(kvStore)
	routerMock := NewMockRouter(testutil.MockCtrl)
	service := NewService(routerMock, webserver.New("localhost:0"))
	return service, kvStore, messageStore, routerMock
}

type TestEndpoint struct {
}

func (*TestEndpoint) GetPrefix() string {
	return "/foo"
}

func (*TestEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "bar")
	return
}
