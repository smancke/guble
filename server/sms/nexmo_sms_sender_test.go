package sms

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/testutil"
	"github.com/stretchr/testify/assert"
)

const (
	KEY    = "ce40b46d"
	SECRET = "153d2b2c72985370"
)

func TestNexmoSender_Send(t *testing.T) {
	a := assert.New(t)
	testutil.SkipIfDisabled(t)
	sender, err := NewNexmoSender(KEY, SECRET)
	a.NoError(err)

	sms := new(NexmoSms)
	sms.To = "+40746278186"
	sms.From = "REWE Lieferservice"
	sms.Text = "Lieber Kunde! Ihre Lieferung kommt heute zwischen 12.04 und 12.34 Uhr. Vielen Dank für Ihre Bestellung! Ihr REWE Lieferservice"

	response, err := sender.sendSms(sms)
	a.Equal(1, response.MessageCount)
	a.Equal(ResponseSuccess, response.Messages[0].Status)
	a.NoError(err)
}

func TestNexmoSender_SendWithError(t *testing.T) {
	RequestTimeout = time.Second
	a := assert.New(t)
	sender, err := NewNexmoSender(KEY, SECRET)
	a.NoError(err)

	sms := NexmoSms{
		To:   "toNumber",
		From: "FromNUmber",
		Text: "body",
	}
	d, err := json.Marshal(&sms)
	a.NoError(err)

	msg := protocol.Message{
		Path:          protocol.Path(SMSDefaultTopic),
		UserID:        "samsa",
		ApplicationID: "sms",
		ID:            uint64(4),
		Body:          d,
	}

	err = sender.Send(&msg)
	a.Error(err)
	a.Equal(ErrIncompleteSMSSent, err)
}
