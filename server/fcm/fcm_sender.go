package fcm

import (
	"encoding/json"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/Bogh/gcm"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/connector"
	"github.com/smancke/guble/server/metrics"
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

func NewSender(apiKey string) *sender {
	return &sender{
		gcmSender: gcm.NewSender(apiKey, sendRetries, sendTimeout),
	}
}

func (s *sender) Send(request connector.Request) (response interface{}, err error) {
	deviceToken := request.Subscriber().Route().Get("device_token")

	fcmMessage := fcmMessage(request.Message())
	fcmMessage.To = deviceToken
	logger.WithFields(log.Fields{"deviceToken": fcmMessage.To}).Debug("sending message")

	beforeSend := time.Now()
	response, err = s.gcmSender.Send(fcmMessage)
	latencyDuration := time.Now().Sub(beforeSend)

	if err != nil && !isValidResponseError(err) {
		// Even if we receive an error we could still have a valid response
		mTotalSentMessageErrors.Add(1)
		metrics.AddToMaps(currentTotalErrorsLatenciesKey, int64(latencyDuration), mMinute, mHour, mDay)
		metrics.AddToMaps(currentTotalErrorsKey, 1, mMinute, mHour, mDay)
		return
	}

	mTotalSentMessages.Add(1)
	metrics.AddToMaps(currentTotalMessagesLatenciesKey, int64(latencyDuration), mMinute, mHour, mDay)
	metrics.AddToMaps(currentTotalMessagesKey, 1, mMinute, mHour, mDay)
	return
}

func fcmMessage(message *protocol.Message) *gcm.Message {
	m := &gcm.Message{}

	err := json.Unmarshal(message.Body, m)
	if err != nil {
		logger.WithFields(log.Fields{
			"error":     err.Error(),
			"body":      string(message.Body),
			"messageID": message.ID,
		}).Debug("Could not decode gcm.Message from guble message body")
	} else if m.Notification != nil && m.Data != nil {
		return m
	}

	err = json.Unmarshal(message.Body, &m.Data)
	if err != nil {
		m.Data = map[string]interface{}{
			"message": message.Body,
		}
	}

	return m
}

// isValidResponseError returns True if the error is accepted as a valid response
// cases are InvalidRegistration and NotRegistered
func isValidResponseError(err error) bool {
	return err.Error() == "InvalidRegistration" || err.Error() == "NotRegistered"
}
