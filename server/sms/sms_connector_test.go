package sms

import (
	"testing"

	"github.com/smancke/guble/server/kvstore"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
	"encoding/json"

	"github.com/smancke/guble/protocol"
	"time"
	"github.com/smancke/guble/server/store/dummystore"
	"github.com/golang/mock/gomock"
)

func Test_StartStop(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer testutil.EnableDebugForMethod()()
	defer finish()
	a := assert.New(t)

	mockSmsSender := NewMockSender(ctrl)
	kvStore := kvstore.NewMemoryKVStore()

	a.NotNil(kvStore)
	routerMock := NewMockRouter(testutil.MockCtrl)
	routerMock.EXPECT().KVStore().AnyTimes().Return(kvStore, nil)

	topic := "sms"
	worker := 1
	config := Config{
		Workers:  &worker,
		SMSTopic: &topic,
		Name:     "test_gateway",
		Schema:   SMSSchema,
	}
	gw, err := New(routerMock, mockSmsSender, config)
	a.NoError(err)

	err = gw.Start()
	a.NoError(err)

	err = gw.Stop()
	a.NoError(err)
}

func Test_SendOneSms(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer testutil.EnableDebugForMethod()()
	defer finish()
	a := assert.New(t)

	mockSmsSender := NewMockSender(ctrl)
	kvStore := kvstore.NewMemoryKVStore()
	//msgStore :=dummystore.New(kvStore)

	a.NotNil(kvStore)
	routerMock := NewMockRouter(testutil.MockCtrl)
	routerMock.EXPECT().KVStore().AnyTimes().Return(kvStore, nil)
	msgStore := dummystore.New(kvStore)
	routerMock.EXPECT().MessageStore().AnyTimes().Return(msgStore,nil)


	topic := "sms"
	worker := 1
	config := Config{
		Workers:  &worker,
		SMSTopic: &topic,
		Name:     "test_gateway",
		Schema:   SMSSchema,
	}

	gw, err := New(routerMock, mockSmsSender, config)
	a.NoError(err)

	err = gw.Start()
	a.NoError(err)

	sms := NexmoSms{
		To:      "toNumber",
		From:    "FromNUmber",
		SmsBody: "body",
	}
	d, err := json.Marshal(&sms)
	a.NoError(err)

	msg := protocol.Message{
		Path: protocol.Path(topic),
		ID: uint64(4),
		Body: d,
	}

	mockSmsSender.EXPECT().Send(gomock.Eq(&msg)).Return(nil)
	a.NotNil(gw.route)
	gw.route.Deliver(&msg)
	time.Sleep(100* time.Millisecond)


	err = gw.Stop()
	a.NoError(err)
}
