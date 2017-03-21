package sms

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_HttpClientRecreation(t *testing.T) {
	a := assert.New(t)

	port := createRandomPort(7000, 8000)
	URL = "http://127.0.0.1" + port

	sender := createNexmoSender(t)
	msg := encodeProtocolMessage(t, 2)

	err := sender.Send(&msg)
	time.Sleep(3 * timeInterval)

	a.Equal(ErrRetryFailed, err)
}

func TestNexmoSender_SendWithError(t *testing.T) {
	RequestTimeout = time.Second
	a := assert.New(t)
	sender, err := NewNexmoSender(KEY, SECRET)
	a.NoError(err)

	msg := encodeProtocolMessage(t, 0)

	err = sender.Send(&msg)
	time.Sleep(3 * timeInterval)
	a.Error(err)
	a.Equal(ErrRetryFailed, err)
}

func TestNexmoSender_SendExpiredMessage(t *testing.T) {
	a := assert.New(t)

	port := createRandomPort(7000, 8000)
	URL = "http://127.0.0.1" + port

	sender := createNexmoSender(t)
	// no request should be made in case the sms is expired
	go dummyNexmoEndpointWithHandlerFunc(t, nil, port, func(t *testing.T, countCh chan bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			a.FailNow("Nexmo call not expected.")
		}
	})

	msg := encodeProtocolMessage(t, 0)
	expires := time.Now().Add(-1 * time.Hour)
	msg.Expires = &expires

	err := sender.Send(&msg)
	time.Sleep(3 * timeInterval)
	a.NoError(err)
}
