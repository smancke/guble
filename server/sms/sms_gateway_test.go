package sms

import (
	"expvar"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/connector"
	"github.com/cosminrentea/gobbler/server/kvstore"
	"github.com/cosminrentea/gobbler/server/router"
	"github.com/cosminrentea/gobbler/server/store/dummystore"
	"github.com/cosminrentea/gobbler/testutil"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func Test_StartStop(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)

	mockSmsSender := NewMockSender(ctrl)
	kvStore := kvstore.NewMemoryKVStore()

	a.NotNil(kvStore)
	routerMock := NewMockRouter(testutil.MockCtrl)
	routerMock.EXPECT().KVStore().AnyTimes().Return(kvStore, nil)

	msgStore := dummystore.New(kvStore)
	routerMock.EXPECT().MessageStore().AnyTimes().Return(msgStore, nil)
	config := createConfig()

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(r *router.Route) (*router.Route, error) {
		a.Equal("sms", r.Path.Partition())
		return r, nil
	}).Times(3)
	routerMock.EXPECT().Unsubscribe(gomock.Any()).Do(func(r *router.Route) {
		a.Equal("sms", r.Path.Partition())
	}).AnyTimes()

	gw, err := New(routerMock, mockSmsSender, config)
	a.NoError(err)

	// try to start & stop
	err = gw.Start()
	a.NoError(err)

	err = gw.Stop()
	a.NoError(err)

	// try to start twice, and then stop twice
	err = gw.Start()
	a.NoError(err)

	err = gw.Start()
	a.NoError(err)

	err = gw.Stop()
	a.NoError(err)

	err = gw.Stop()
	a.NoError(err)

	// try to start & stop once again
	err = gw.Start()
	a.NoError(err)

	err = gw.Stop()
	a.NoError(err)

	time.Sleep(100 * time.Millisecond)
}

func Test_SendOneSms(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	a := assert.New(t)

	mockSmsSender := NewMockSender(ctrl)
	kvStore := kvstore.NewMemoryKVStore()

	a.NotNil(kvStore)
	routerMock := NewMockRouter(testutil.MockCtrl)
	routerMock.EXPECT().KVStore().AnyTimes().Return(kvStore, nil)
	msgStore := dummystore.New(kvStore)
	routerMock.EXPECT().MessageStore().AnyTimes().Return(msgStore, nil)

	config := createConfig()

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(r *router.Route) (*router.Route, error) {
		a.Equal(*config.SMSTopic, string(r.Path))
		return r, nil
	})
	routerMock.EXPECT().Unsubscribe(gomock.Any()).Do(func(r *router.Route) {
		a.Equal(*config.SMSTopic, string(r.Path))
	}).AnyTimes()

	gw, err := New(routerMock, mockSmsSender, config)
	a.NoError(err)

	err = gw.Start()
	a.NoError(err)
	msg := encodeProtocolMessage(t, 4, "body")

	mockSmsSender.EXPECT().Send(gomock.Eq(&msg)).Return(nil)
	a.NotNil(gw.route)
	gw.route.Deliver(&msg, true)
	time.Sleep(100 * time.Millisecond)
	err = gw.Stop()
	a.NoError(err)

	err = gw.ReadLastID()
	a.NoError(err)

	time.Sleep(100 * time.Millisecond)

	totalSentCount := expvar.NewInt("total_sent_messages")
	totalSentCount.Add(1)
	a.Equal(totalSentCount, mTotalSentMessages)
}

func Test_Restart(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	defer testutil.EnableDebugForMethod()()
	a := assert.New(t)

	mockSmsSender := NewMockSender(ctrl)
	kvStore := kvstore.NewMemoryKVStore()

	a.NotNil(kvStore)
	routerMock := NewMockRouter(testutil.MockCtrl)
	routerMock.EXPECT().KVStore().AnyTimes().Return(kvStore, nil)
	msgStore := NewMockMessageStore(ctrl)
	routerMock.EXPECT().MessageStore().AnyTimes().Return(msgStore, nil)

	config := createConfig()

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(r *router.Route) (*router.Route, error) {
		a.Equal(strings.Split(*config.SMSTopic, "/")[1], r.Path.Partition())
		return r, nil
	}).Times(2)

	gw, err := New(routerMock, mockSmsSender, config)
	a.NoError(err)

	err = gw.Start()
	a.NoError(err)

	msg := encodeProtocolMessage(t, 4, "body")

	mockSmsSender.EXPECT().Send(gomock.Eq(&msg)).Times(1).Return(connector.ErrRouteChannelClosed)

	doneC := make(chan bool)
	routerMock.EXPECT().Done().AnyTimes().Return(doneC)

	a.NotNil(gw.route)
	gw.route.Deliver(&msg, true)
	time.Sleep(100 * time.Millisecond)
}

func TestReadLastID(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	defer testutil.EnableDebugForMethod()()
	a := assert.New(t)

	mockSmsSender := NewMockSender(ctrl)
	kvStore := kvstore.NewMemoryKVStore()

	a.NotNil(kvStore)
	routerMock := NewMockRouter(testutil.MockCtrl)
	routerMock.EXPECT().KVStore().AnyTimes().Return(kvStore, nil)
	msgStore := dummystore.New(kvStore)
	routerMock.EXPECT().MessageStore().AnyTimes().Return(msgStore, nil)

	config := createConfig()

	gw, err := New(routerMock, mockSmsSender, config)
	a.NoError(err)

	gw.SetLastSentID(uint64(10))

	gw.ReadLastID()

	a.Equal(uint64(10), gw.LastIDSent)
}

func Test_SmsRouteProvideError(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	defer testutil.EnableDebugForMethod()()
	a := assert.New(t)

	//create a mockSms Sender to simulate Nexmo error
	mockSmsSender := NewMockSender(ctrl)
	f := tempFilename("guble_test_retry_sms_loop")
	defer os.Remove(f)

	//create a KVStore with sqlite3
	kvStore := kvstore.NewSqliteKVStore(f, false)
	err := kvStore.Open()
	a.NoError(err)
	a.NotNil(kvStore)

	//create a new mock router
	routerMock := NewMockRouter(testutil.MockCtrl)
	routerMock.EXPECT().KVStore().AnyTimes().Return(kvStore, nil)

	//create a new mock msg store
	mockMessageStore := NewMockMessageStore(ctrl)
	routerMock.EXPECT().MessageStore().AnyTimes().Return(mockMessageStore, nil)

	//setup a new sms gateway
	config := createConfig()
	gateway, err := New(routerMock, mockSmsSender, config)
	a.NoError(err)

	//create a new route on which the gateway will subscribe on /sms
	route := router.NewRoute(router.RouteConfig{
		Path:         protocol.Path(*gateway.config.SMSTopic),
		ChannelSize:  5000,
		FetchRequest: gateway.fetchRequest(),
	})

	//expect 2 subscribe.
	// One for Start which will return a ErrInvalidRoute
	routerMock.EXPECT().Subscribe(gomock.Any()).Times(1).Return(route, router.ErrInvalidRoute)
	//second will be done for restart caused by the subscribed which failed
	routerMock.EXPECT().Subscribe(gomock.Any()).Times(1).Return(route, nil)

	err = gateway.Start()
	a.NoError(err)

	time.Sleep(timeInterval)
	err = gateway.Stop()
	a.NoError(err)
}

func Test_RetryLoop(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	defer testutil.EnableDebugForMethod()()
	a := assert.New(t)

	//create a mockSms Sender to simulate Nexmo error
	mockSmsSender := NewMockSender(ctrl)
	f := tempFilename("guble_test_retry_sms_loop")
	defer os.Remove(f)

	//create a KVStore with sqlite3
	kvStore := kvstore.NewSqliteKVStore(f, false)
	err := kvStore.Open()
	a.NoError(err)
	a.NotNil(kvStore)

	//create a new mock router
	routerMock := NewMockRouter(testutil.MockCtrl)
	routerMock.EXPECT().KVStore().AnyTimes().Return(kvStore, nil)

	//create a new mock msg store
	mockMessageStore := NewMockMessageStore(ctrl)
	routerMock.EXPECT().MessageStore().AnyTimes().Return(mockMessageStore, nil)

	//setup a new sms gateway
	worker := 1
	topic := SMSDefaultTopic
	enableMetrics := false
	skipFetch := false
	gateway, err := New(routerMock, mockSmsSender,
		Config{
			Workers:         &worker,
			Name:            SMSDefaultTopic,
			Schema:          SMSSchema,
			SMSTopic:        &topic,
			Toggleable:      &skipFetch,
			IntervalMetrics: &enableMetrics,
		})
	a.NoError(err)

	//create a new route on which the gateway will subscribe on /sms
	route := router.NewRoute(router.RouteConfig{
		Path:         protocol.Path(*gateway.config.SMSTopic),
		ChannelSize:  5000,
		FetchRequest: gateway.fetchRequest(),
	})
	//expect 2 subscribe.One for Start.One for Restart
	routerMock.EXPECT().Subscribe(gomock.Any()).Times(1).Return(route, nil)

	err = gateway.Start()
	a.NoError(err)

	msg := encodeProtocolMessage(t, 4, "body")

	// when sms is sent simulate an error.No Sms will be delivered with success for 4 times.
	mockSmsSender.EXPECT().Send(gomock.Eq(&msg)).Times(1).Return(ErrRetryFailed)
	a.NotNil(gateway.route)

	// deliver the message on the gateway route.
	gateway.route.Deliver(&msg, true)
	// wait for retry to complete alongside with the 3 sent sms.
	time.Sleep(4 * timeInterval)
	a.Equal(uint64(4), gateway.LastIDSent, "Last id sent should be the msgID since the retry failed.Already try it for 4 time.Anymore retries would be useless")

	//Send a new message afterwards.
	successMsg := encodeProtocolMessage(t, 9, "body")

	//no error will be raised for second message.
	mockSmsSender.EXPECT().Send(gomock.Eq(&successMsg)).Times(1).Return(nil)
	gateway.route.Deliver(&successMsg, true)
	//wait for db to save the last id.
	time.Sleep(timeInterval)
	a.Equal(uint64(9), gateway.LastIDSent, "Last id sent should be successMsgId")
}
