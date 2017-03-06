package sms

import (
	"testing"

	"encoding/json"

	"github.com/smancke/guble/server/kvstore"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"

	"strings"
	"time"

	"expvar"
	"github.com/golang/mock/gomock"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/router"
	"github.com/smancke/guble/server/store/dummystore"
	"io/ioutil"
	"os"
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
	topic := "sms"
	worker := 1
	intervalMetrics := true
	config := Config{
		Workers:         &worker,
		SMSTopic:        &topic,
		Name:            "test_gateway",
		Schema:          SMSSchema,
		IntervalMetrics: &intervalMetrics,
	}

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(r *router.Route) (*router.Route, error) {
		a.Equal(topic, r.Path.Partition())
		return r, nil
	})

	gw, err := New(routerMock, mockSmsSender, config)
	a.NoError(err)

	err = gw.Start()
	a.NoError(err)

	err = gw.Stop()
	a.NoError(err)

	time.Sleep(100 * time.Millisecond)
}

func Test_SendOneSms(t *testing.T) {
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

	topic := "/sms"
	worker := 1
	intervalMetrics := true
	config := Config{
		Workers:         &worker,
		SMSTopic:        &topic,
		Name:            "test_gateway",
		Schema:          SMSSchema,
		IntervalMetrics: &intervalMetrics,
	}

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(r *router.Route) (*router.Route, error) {
		a.Equal(topic, string(r.Path))
		return r, nil
	})

	gw, err := New(routerMock, mockSmsSender, config)
	a.NoError(err)

	err = gw.Start()
	a.NoError(err)

	sms := NexmoSms{
		To:   "toNumber",
		From: "FromNUmber",
		Text: "body",
	}
	d, err := json.Marshal(&sms)
	a.NoError(err)

	msg := protocol.Message{
		Path: protocol.Path(topic),
		ID:   uint64(4),
		Body: d,
	}

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

	topic := "/sms"
	worker := 1
	intervalMetrics := true
	config := Config{
		Workers:  &worker,
		SMSTopic: &topic,
		Name:     "test_gateway",
		Schema:   SMSSchema,

		IntervalMetrics: &intervalMetrics,
	}

	routerMock.EXPECT().Subscribe(gomock.Any()).Do(func(r *router.Route) (*router.Route, error) {
		a.Equal(strings.Split(topic, "/")[1], r.Path.Partition())
		return r, nil
	}).Times(2)

	gw, err := New(routerMock, mockSmsSender, config)
	a.NoError(err)

	err = gw.Start()
	a.NoError(err)

	sms := NexmoSms{
		To:   "toNumber",
		From: "FromNUmber",
		Text: "body",
	}
	d, err := json.Marshal(&sms)
	a.NoError(err)

	msg := protocol.Message{
		Path:          protocol.Path(topic),
		UserID:        "samsa",
		ApplicationID: "sms",
		ID:            uint64(4),
		Body:          d,
	}

	mockSmsSender.EXPECT().Send(gomock.Eq(&msg)).Times(1).Return(ErrNoSMSSent)

	doneC := make(chan bool)
	routerMock.EXPECT().Done().AnyTimes().Return(doneC)
	//
	//mockSmsSender.EXPECT().Send(gomock.Eq(&msg)).Return(nil)

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

	topic := "/sms"
	worker := 1
	config := Config{
		Workers:  &worker,
		SMSTopic: &topic,
		Name:     "test_gateway",
		Schema:   SMSSchema,
	}

	gw, err := New(routerMock, mockSmsSender, config)
	a.NoError(err)

	gw.SetLastSentID(uint64(10))

	gw.ReadLastID()

	a.Equal(uint64(10), gw.LastIDSent)
}

func tempFilename() string {
	file, err := ioutil.TempFile("/tmp", "guble_sms_retry")
	if err != nil {
		panic(err)
	}
	file.Close()
	os.Remove(file.Name())
	return file.Name()
}

func Test_RetryLoop(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()

	defer testutil.EnableDebugForMethod()()
	a := assert.New(t)

	//create a mockSms Sender to simulate Nexmo error
	mockSmsSender := NewMockSender(ctrl)
	f := tempFilename()
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
	mockMessageStore.EXPECT().MaxMessageID(gomock.Any()).Return(uint64(2), nil)

	//setup a new sms gateway
	worker := 8
	topic := SMSDefaultTopic
	enableMetrics := false
	gateway, err := New(routerMock, mockSmsSender, Config{Workers: &worker, Name: SMSDefaultTopic, Schema: SMSSchema, SMSTopic: &topic, IntervalMetrics: &enableMetrics})
	a.NoError(err)

	//create a new route on which the gateway will subscribe on /sms
	route := router.NewRoute(router.RouteConfig{
		Path:         protocol.Path(*gateway.config.SMSTopic),
		ChannelSize:  5000,
		FetchRequest: gateway.fetchRequest(),
	})
	//expect 2 subscribe.One for Start.One for Restart
	routerMock.EXPECT().Subscribe(gomock.Any()).Times(2).Return(route, nil)

	err = gateway.Start()
	a.NoError(err)

	//create a new sms message
	sms := NexmoSms{
		To:   "toNumber",
		From: "FromNUmber",
		Text: "body",
	}
	d, err := json.Marshal(&sms)
	a.NoError(err)

	msg := protocol.Message{
		Path:          protocol.Path(topic),
		UserID:        "samsa",
		ApplicationID: "sms",
		ID:            uint64(4),
		Body:          d,
	}

	// when sms is sent simulate an error.No Sms will be delivered with success for 4 times.
	mockSmsSender.EXPECT().Send(gomock.Eq(&msg)).Times(4).Return(ErrIncompleteSMSSent)
	a.NotNil(gateway.route)

	// deliver the message on the gateway route.
	gateway.route.Deliver(&msg, true)
	// wait for retry to complete alongside with the 3 sent sms.
	time.Sleep(500 * time.Millisecond)
	a.Equal(uint64(4), gateway.LastIDSent, "Last id sent should be the msgID since the retry failed.Already try it for 4 time.Anymore retries would be useless")

	//Send a new message afterwards.
	successMsg := protocol.Message{
		Path:          protocol.Path(topic),
		UserID:        "samsa",
		ApplicationID: "sms",
		ID:            uint64(9),
		Body:          d,
	}

	//no error will be raised for second message.
	mockSmsSender.EXPECT().Send(gomock.Eq(&successMsg)).Times(1).Return(nil)
	gateway.route.Deliver(&successMsg, true)
	//wait for db to save the last id.
	time.Sleep(100 * time.Millisecond)
	a.Equal(uint64(9), gateway.LastIDSent, "Last id sent should be successMsgId")

}
