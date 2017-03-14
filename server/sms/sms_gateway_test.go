package sms

import (
	"testing"

	"encoding/json"

	"github.com/cosminrentea/gobbler/server/kvstore"
	"github.com/cosminrentea/gobbler/testutil"
	"github.com/stretchr/testify/assert"

	"strings"
	"time"

	"expvar"
	"github.com/golang/mock/gomock"
	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/router"
	"github.com/cosminrentea/gobbler/server/store/dummystore"
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
