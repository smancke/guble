package fcm

import (
	"time"

	"github.com/Bogh/gcm"
)

const (
	// sendRetries is the number of retries when something fails
	sendRetries = 5

	// sendTimeout timeout to wait for response from FCM
	sendTimeout = time.Second
)

type sender struct {
	gcmSender gcm.Sender
}

func NewSender(apiKey string) connector.Sender,
