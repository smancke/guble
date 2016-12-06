package sms

import (
	"github.com/smancke/guble/server/kvstore"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
	//"github.com/smancke/guble/server/store/dummystore"
	"github.com/smancke/guble/protocol"
	"encoding/json"
)

func TestGateway_StartStop(t *testing.T) {
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




func TestGateway_Run(t *testing.T) {
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
	//routerMock.EXPECT().MessageStore().AnyTimes().Return(msgStore,nil)

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
		To:"toNumber",
		From:"FromNUmber",
		SmsBody:"boyd",
	}
	d, err := json.Marshal(&sms)
	a.NoError(err)

	msg:= protocol.Message{
		  Path:  protocol.Path(topic),
		Body:        d,
	}
	a.NotNil(gw.route)
	logger.WithField("msg",msg).Info("aaaaaaaaaaaaaa")
	gw.route.Deliver(&msg)


}

