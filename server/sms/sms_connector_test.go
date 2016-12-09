package sms

import (
	"testing"

	"encoding/json"

	"github.com/smancke/guble/server/kvstore"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"

	"strings"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/store"
	"github.com/smancke/guble/server/store/dummystore"
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
	gw.route.Deliver(&msg)
	time.Sleep(100 * time.Millisecond)

	err = gw.Stop()
	a.NoError(err)

	err = gw.ReadLastID()
	a.NoError(err)

	time.Sleep(100 * time.Millisecond)
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

	msgStore.EXPECT().MaxMessageID(gomock.Eq(gw.route.Path.Partition())).Return(uint64(0), nil)
	msgStore.EXPECT().MaxMessageID(gomock.Eq(gw.route.Path.Partition())).Return(uint64(4), nil)
	msgStore.EXPECT().MaxMessageID(gomock.Eq(gw.route.Path.Partition())).Return(uint64(4), nil)
	mockSmsSender.EXPECT().Send(gomock.Eq(&msg)).Times(1).Return(ErrNoSMSSent)

	routerMock.EXPECT().Fetch(gomock.Any()).Do(func(r *store.FetchRequest) {
		go func() {

			logger.WithField("r.Partition", r.Partition).Info("----")

			a.Equal(strings.Split(topic, "/")[1], r.Partition)

			r.StartC <- 1

			r.MessageC <- &store.FetchedMessage{ID: uint64(4), Message: msg.Bytes()}
			close(r.MessageC)
		}()
	})
	doneC := make(chan bool)
	routerMock.EXPECT().Done().AnyTimes().Return(doneC)

	mockSmsSender.EXPECT().Send(gomock.Eq(&msg)).Return(nil)

	a.NotNil(gw.route)
	gw.route.Deliver(&msg)
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
