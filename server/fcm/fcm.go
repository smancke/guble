package fcm

import (
	"time"

	"github.com/Bogh/gcm"
	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/connector"
	"github.com/smancke/guble/server/metrics"
	"github.com/smancke/guble/server/router"
)

const (
	// schema is the default database schema for FCM
	schema = "fcm_registration"

	// sendRetries is the number of retries when sending a message
	sendRetries = 5

	sendTimeout = time.Second

	subscribePrefixPath = "subscribe"

	bufferSize = 1000

	routeChannelSize = 100

	syncPath = protocol.Path("/fcm/sync")
)

// Config is used for configuring the Firebase Cloud Messaging component.
type Config struct {
	Enabled              *bool
	APIKey               *string
	Workers              *int
	Endpoint             *string
	AfterMessageDelivery protocol.MessageDeliveryCallback
}

// Connector is the structure for handling the communication with Firebase Cloud Messaging
type Connector struct {
	Config
	connector.Connector
}

// New creates a new *Connector without starting it
func New(router router.Router, prefix string, config Config) (*Connector, error) {
	baseConn, err :=
}

// Check returns nil if health-check succeeds, or an error if health-check fails
// by sending a request with only apikey. If the response is processed by the FCM endpoint
// the status will be UP, otherwise the error from sending the message will be returned.
func (conn *Connector) check() error {
	message := &gcm.Message{
		To: "ABC",
	}
	_, err := conn.Sender.Send(message)
	if err != nil {
		logger.WithField("error", err.Error()).Error("error checking FCM connection")
		return err
	}
	return nil
}

func (conn *Connector) Send(pm *subscriptionMessage) {
	fcmID := pm.subscription.route.Get(applicationIDKey)

	fcmMessage := pm.fcmMessage()
	fcmMessage.To = fcmID
	logger.WithFields(log.Fields{
		"fcmTo":      fcmMessage.To,
		"pipeLength": len(conn.pipelineC),
	}).Debug("sending message")

	beforeSend := time.Now()
	response, err := conn.Sender.Send(fcmMessage)
	latencyDuration := time.Now().Sub(beforeSend)

	if err != nil && !isValidResponseError(err) {
		// Even if we receive an error we could still have a valid response
		pm.errC <- err
		mTotalSentMessageErrors.Add(1)
		metrics.AddToMaps(currentTotalErrorsLatenciesKey, int64(latencyDuration), mMinute, mHour, mDay)
		metrics.AddToMaps(currentTotalErrorsKey, 1, mMinute, mHour, mDay)
		return
	}
	mTotalSentMessages.Add(1)
	metrics.AddToMaps(currentTotalMessagesLatenciesKey, int64(latencyDuration), mMinute, mHour, mDay)
	metrics.AddToMaps(currentTotalMessagesKey, 1, mMinute, mHour, mDay)

	if conn.AfterMessageDelivery != nil {
		go conn.AfterMessageDelivery(pm.message)
	}
	pm.resultC <- response
}

// isValidResponseError returns True if the error is accepted as a valid response
// cases are InvalidRegistration and NotRegistered
func isValidResponseError(err error) bool {
	return err.Error() == "InvalidRegistration" || err.Error() == "NotRegistered"
}
