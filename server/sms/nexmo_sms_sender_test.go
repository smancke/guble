package sms

import (
	"testing"
	"time"

	"encoding/json"
	"fmt"

	"github.com/cosminrentea/gobbler/testutil"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"strings"
)

func Test_HttpClientRecreation(t *testing.T) {
	a := assert.New(t)

	port := createRandomPort(7000, 8000)
	URL = "http://127.0.0.1" + port

	sender := createNexmoSender(t)
	msg := encodeProtocolMessage(t, 2, "body")

	err := sender.Send(&msg)
	time.Sleep(3 * timeInterval)

	a.Equal(ErrRetryFailed, err)
}

func TestNexmoSender_SendWithError(t *testing.T) {
	defer testutil.EnableDebugForMethod()
	RequestTimeout = time.Second
	a := assert.New(t)
	sender, err := NewNexmoSender(KEY, SECRET, nil, "")
	a.NoError(err)

	msg := encodeProtocolMessage(t, 0, "body")

	err = sender.Send(&msg)
	time.Sleep(3 * timeInterval)
	a.Error(err)
	a.Equal(ErrRetryFailed, err)
}

func TestNexmoSender_SendSmsTooLong(t *testing.T) {
	defer testutil.EnableDebugForMethod()
	RequestTimeout = time.Second
	a := assert.New(t)
	sender, err := NewNexmoSender(KEY, SECRET, nil, "")
	a.NoError(err)

	msgTooLong := encodeProtocolMessage(t, 0, strings.Repeat("x",161))

	err = sender.Send(&msgTooLong)
	a.Equal(ErrSmsTooLong, err)

	msgMaxLength := encodeProtocolMessage(t, 0, strings.Repeat("x",160))

	err = sender.Send(&msgMaxLength)
	a.Equal(ErrRetryFailed, err)
}

func TestNexmoSender_SendReport(t *testing.T) {
	ctrl, finish := testutil.NewMockCtrl(t)
	defer finish()
	a := assert.New(t)
	defer testutil.EnableDebugForMethod()()

	port := createRandomPort(7000, 9000)
	URL = "http://127.0.0.1" + port

	countCh := make(chan bool, 0)
	go dummyNexmoEndpointWithHandlerFunc(t, countCh, port, succesSenderNexmoHandler)
	producer := NewMockProducer(ctrl)

	nexmoSender, err := NewNexmoSender(KEY, SECRET, producer, "sms_reporting")
	if err != nil {
		a.FailNow("Nexmo sender could not be created.")
	}

	msg := encodeProtocolMessage(t, 0, "body")
	producer.EXPECT().Report(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(topic string, bytes []byte, key string) {
		a.Equal("sms_reporting", topic)

		var event ReportEvent
		err := json.Unmarshal(bytes, &event)
		a.NoError(err)
		fmt.Println(event)
		a.Equal(event.Id, key)
		a.Equal("ref", event.Payload.OrderID)
		a.Equal("tour_arrival_estimate_regular_delivered", event.Type)
		a.Equal("toNumber", event.Payload.MobileNumber)
		a.Equal("Success", event.Payload.DeliveryStatus)
		a.Equal("7sdks723ksgqn", event.Payload.MessageID)
		a.Equal("body", event.Payload.SmsText)

	})
	err = nexmoSender.Send(&msg)
	a.NoError(err)
	readServedResponses(t, 1, countCh)
}
