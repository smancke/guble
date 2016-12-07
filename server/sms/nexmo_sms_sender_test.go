package sms

import (
	"encoding/json"
	"github.com/smancke/guble/protocol"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNexmoSender_Send(t *testing.T) {
	a := assert.New(t)
	sender, err := NewNexmoSender(KEY, SECRET)
	a.NoError(err)

	sms := new(NexmoSms)
	sms.To = "+40746278186"
	sms.From = "REWE Lieferservice"
	sms.SmsBody = "Lieber Kunde! Ihre Lieferung kommt heute zwischen 12.04 und 12.34 Uhr. Vielen Dank für Ihre Bestellung! Ihr REWE Lieferservice"

	response, err := sender.SendSms(sms)
	a.Equal(1, response.MessageCount)
	a.Equal(ResponseSuccess, response.Messages[0].Status)
	a.NoError(err)
}

func TestNexmoSender_SendWithError(t *testing.T) {
	a := assert.New(t)
	sender, err := NewNexmoSender(KEY, SECRET)
	a.NoError(err)

	sms := NexmoSms{
		To:      "toNumber",
		From:    "FromNUmber",
		SmsBody: "body",
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
	a.Equal(ErrIncompleteSMSSent,err)
}
