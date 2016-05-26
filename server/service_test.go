package server

import (
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/store"
	"github.com/stretchr/testify/assert"

	"fmt"
	"github.com/docker/distribution/health"
	"net/http"
	"testing"
	"time"
)

func TestStopingOfModules(t *testing.T) {
	defer initCtrl(t)()

	// reset existing health-checks
	health.DefaultRegistry = health.NewRegistry()

	// given:
	service, _, _, _ := aMockedService()

	// with a registered Stopable
	stopable := NewMockStopable(ctrl)
	service.Register(stopable)

	service.Start()

	// when i stop the service, the Stop() is called
	stopable.EXPECT().Stop()
	service.Stop()
}

func TestStopingOfModulesTimeout(t *testing.T) {
	defer initCtrl(t)()

	// reset existing health-checks
	health.DefaultRegistry = health.NewRegistry()

	// given:
	service, _, _, _ := aMockedService()
	service.StopGracePeriod = time.Millisecond * 5

	// with a registered Stopable, which blocks too long on stop
	stopable := NewMockStopable(ctrl)
	service.Register(stopable)
	stopable.EXPECT().Stop().Do(func() {
		time.Sleep(time.Millisecond * 10)
	})

	// then the Stop returns with an error
	err := service.Stop()
	assert.Error(t, err)
	protocol.Err(err.Error())
}

func TestEndpointRegisterAndServing(t *testing.T) {
	defer initCtrl(t)()

	// reset existing health-checks
	health.DefaultRegistry = health.NewRegistry()

	// given:
	service, _, _, _ := aMockedService()

	// when I register an endpoint at path /foo
	service.Register(&TestEndpoint{})
	service.Start()
	defer service.Stop()
	time.Sleep(time.Millisecond * 10)

	// then I can call the handler
	url := fmt.Sprintf("http://%s/foo", service.WebServer().GetAddr())
	result, err := http.Get(url)
	assert.NoError(t, err)
	body := make([]byte, 3)
	result.Body.Read(body)
	assert.Equal(t, "bar", string(body))
}

func TestHealthUp(t *testing.T) {
	defer initCtrl(t)()

	// reset existing health-checks
	health.DefaultRegistry = health.NewRegistry()

	// given:
	service, _, _, _ := aMockedService()

	// when starting the service
	defer service.Stop()
	service.Start()
	time.Sleep(time.Millisecond * 10)

	// then I can call the health URL
	url := fmt.Sprintf("http://%s/health", service.WebServer().GetAddr())
	result, err := http.Get(url)
	assert.NoError(t, err)
	body := make([]byte, 3)
	result.Body.Read(body)
	assert.Equal(t, "{}\x00", string(body))
}

func aMockedService() (*Service, store.KVStore, store.MessageStore, *MockRouter) {
	kvStore := store.NewMemoryKVStore()
	messageStore := store.NewDummyMessageStore(kvStore)
	routerMock := NewMockRouter(ctrl)
	service := NewService("localhost:0", routerMock)
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
