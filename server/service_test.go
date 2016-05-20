package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"
	"github.com/stretchr/testify/assert"

	"fmt"
	"github.com/smancke/guble/server/auth"
	"net/http"
	"testing"
	"time"
)

func TestStopingOfModules(t *testing.T) {
	defer initCtrl(t)()
	// given:
	service, _, _, _, _ := aMockedService()

	// whith a registered stopable
	stopable := NewMockStopable(ctrl)
	service.Register(stopable)

	service.Start()

	// when i stop the service,
	// the stopable is called
	stopable.EXPECT().Stop()
	service.Stop()
}

func TestStopingOfModulesTimeout(t *testing.T) {
	defer initCtrl(t)()

	// given:
	service, _, _, _, _ := aMockedService()
	service.StopGracePeriod = time.Millisecond * 5

	// whith a registered stopable, which blocks to long on stop
	stopable := NewMockStopable(ctrl)
	service.Register(stopable)
	stopable.EXPECT().Stop().Do(func() {
		time.Sleep(time.Millisecond * 10)
	})

	// then the Stop returns with an error
	err := service.Stop()
	assert.Error(t, err)
	guble.Err(err.Error())
}

func TestRegistrationOfSetter(t *testing.T) {
	defer initCtrl(t)()

	// given:
	service, _, _, messageSink, _ := aMockedService()
	setMessageEntryMock := NewMockSetMessageEntry(ctrl)

	// then I expect
	setMessageEntryMock.EXPECT().SetMessageEntry(messageSink)

	// when I register the modules
	service.Register(setMessageEntryMock)
}

func TestEndpointRegisterAndServing(t *testing.T) {
	defer initCtrl(t)()

	// given:
	service, _, _, _, _ := aMockedService()

	// when I register an endpoint at path /foo
	service.Register(&TestEndpoint{})
	service.Start()
	defer service.Stop()
	time.Sleep(time.Millisecond * 10)

	// then I can call the handler
	url := fmt.Sprintf("http://%s/foo", service.GetWebServer().GetAddr())
	result, err := http.Get(url)
	assert.NoError(t, err)
	body := make([]byte, 3)
	result.Body.Read(body)
	assert.Equal(t, "bar", string(body))
}

func aMockedService() (*Service, store.KVStore, store.MessageStore, *MockMessageSink, *MockPubSubSource) {
	kvStore := store.NewMemoryKVStore()
	messageStore := store.NewDummyMessageStore()
	messageSink := NewMockMessageSink(ctrl)
	pubSubSource := NewMockPubSubSource(ctrl)
	return NewService("localhost:0", kvStore, messageStore, messageSink, pubSubSource, auth.NewAllowAllAccessManager(true)),
		kvStore,
		messageStore,
		messageSink,
		pubSubSource

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
