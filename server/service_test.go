package server

import (
	"github.com/smancke/guble/guble"
	"github.com/stretchr/testify/assert"

	"testing"
	"time"
)

func TestStopingOfModules(t *testing.T) {
	defer initCtrl(t)()

	// given:
	service, _, _ := aMockedService()

	// whith a registered stopable
	stopable := NewMockStopable(ctrl)
	service.Register(stopable)

	// when i stop the service,
	// the stopable is called
	stopable.EXPECT().Stop()
	service.Stop()
}

func TestStopingOfModulesTimeout(t *testing.T) {
	defer initCtrl(t)()

	// given:
	service, _, _ := aMockedService()
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
	service, router, messageSink := aMockedService()
	setRouterMock := NewMockSetRouter(ctrl)
	setMessageEntryMock := NewMockSetMessageEntry(ctrl)

	// then I expect
	setRouterMock.EXPECT().SetRouter(router)
	setMessageEntryMock.EXPECT().SetMessageEntry(messageSink)

	// when I register the modules
	service.Register(setRouterMock)
	service.Register(setMessageEntryMock)
}

func aMockedService() (*Service, *MockPubSubSource, *MockMessageSink) {
	pubSubSource := NewMockPubSubSource(ctrl)
	messageSink := NewMockMessageSink(ctrl)
	return NewService("localhost:0", pubSubSource, messageSink),
		pubSubSource,
		messageSink
}
