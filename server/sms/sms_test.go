package gateway

import (
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNexmoSender_Send(t *testing.T) {
	defer testutil.EnableDebugForMethod()()
	a := assert.New(t)
	sender, err := NewNexmoSender(KEY, SECRET)
	a.NoError(err)

	response, err := sender.SendSms(
		"40746278186",
		"REWE Lieferservice",
		"Lieber Kunde! Ihre Lieferung kommt heute zwischen 12.04 und 12.34 Uhr. Vielen Dank für Ihre Bestellung! Ihr REWE Lieferservice",
	)
	a.Equal(1, response.MessageCount)
	a.Equal(ResponseSuccess, response.Messages[0].Status)
	a.NoError(err)
}
