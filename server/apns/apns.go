package apns

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/sideshow/apns2"
	"github.com/smancke/guble/server/connector"
	"github.com/smancke/guble/server/router"
)

const (
	// schema is the default database schema for APNS
	schema = "apns_registration"

	errNotSentMsg = "APNS notification was not sent"
)

// Config is used for configuring the APNS module.
type Config struct {
	Enabled             *bool
	Production          *bool
	CertificateFileName *string
	CertificateBytes    *[]byte
	CertificatePassword *string
	AppTopic            *string
	Workers             *int
}

// conn is the private struct for handling the communication with APNS
type conn struct {
	*connector.Conn
}

// New creates a new Connector without starting it
func New(router router.Router, prefix string, config Config) (connector.Connector, error) {
	sender, err := newSender(config)
	if err != nil {
		log.WithError(err).Error("APNS Sender error")
		return nil, err
	}
	connectorConfig := connector.Config{
		Name:       "apns",
		Schema:     schema,
		Prefix:     prefix,
		URLPattern: fmt.Sprintf("/{device_token}/{user_id}/{%s:.*}", connector.TopicParam),
	}
	baseConn, err := connector.NewConnector(router, sender, connectorConfig)
	if err != nil {
		log.WithError(err).Error("Base connector error")
		return nil, err
	}
	newConn := &conn{baseConn}
	newConn.SetResponseHandler(newConn)
	return newConn, nil
}

func (c *conn) HandleResponse(request connector.Request, responseIface interface{}, errSend error) error {
	log.Debug("HandleResponse")
	if errSend != nil {
		logger.WithError(errSend).Error("error when trying to send APNS notification")
		return errSend
	}
	if r, ok := responseIface.(*apns2.Response); ok {
		messageID := request.Message().ID
		if err := request.Subscriber().SetLastID(messageID); err != nil {
			logger.WithError(errSend).Error("error when setting the last-id for the subscriber")
			return err
		}
		if r.Sent() {
			log.WithField("id", r.ApnsID).Debug("APNS notification was successfully sent")
			return nil
		}
		log.WithField("id", r.ApnsID).WithField("reason", r.Reason).Error(errNotSentMsg)
		switch r.Reason {
		case
			apns2.ReasonMissingDeviceToken,
			apns2.ReasonBadDeviceToken,
			apns2.ReasonDeviceTokenNotForTopic,
			apns2.ReasonUnregistered:
			// TODO Cosmin Bogdan remove subscription (+Unsubscribe and stop subscription loop) ?
		}
		//TODO Cosmin Bogdan: extra-APNS-handling
	}
	return nil
}

// Check returns nil if health-check succeeds, or an error if health-check fails
func (c *conn) Check() error {
	//TODO implement
	return nil
}
