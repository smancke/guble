package apns

import (
	"fmt"
	"github.com/sideshow/apns2"
	"github.com/smancke/guble/server/connector"
	"github.com/smancke/guble/server/router"
)

const (
	// schema is the default database schema for APNS
	schema = "apns_registration"
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
	Prefix              *string
}

// conn is the private struct for handling the communication with APNS
type conn struct {
	Config
	connector.Connector
}

// New creates a new Connector without starting it
func New(router router.Router, sender connector.Sender, config Config) (connector.ReactiveConnector, error) {
	baseConn, err := connector.NewConnector(
		router,
		sender,
		connector.Config{
			Name:       "apns",
			Schema:     schema,
			Prefix:     *config.Prefix,
			URLPattern: fmt.Sprintf("/{device_token}/{user_id}/{%s:.*}", connector.TopicParam),
			Workers:    *config.Workers,
		},
	)
	if err != nil {
		logger.WithError(err).Error("Base connector error")
		return nil, err
	}
	newConn := &conn{
		Config:    config,
		Connector: baseConn,
	}
	newConn.SetResponseHandler(newConn)
	return newConn, nil
}

func (c *conn) HandleResponse(request connector.Request, responseIface interface{}, errSend error) error {
	logger.Debug("HandleResponse")
	if errSend != nil {
		logger.WithError(errSend).Error("error when trying to send APNS notification")
		return errSend
	}
	if r, ok := responseIface.(*apns2.Response); ok {
		messageID := request.Message().ID
		subscriber := request.Subscriber()
		subscriber.SetLastID(messageID)
		if r.Sent() {
			logger.WithField("id", r.ApnsID).Debug("APNS notification was successfully sent")
			return nil
		}
		logger.WithField("id", r.ApnsID).WithField("reason", r.Reason).Error("APNS notification was not sent")
		switch r.Reason {
		case
			apns2.ReasonMissingDeviceToken,
			apns2.ReasonBadDeviceToken,
			apns2.ReasonDeviceTokenNotForTopic,
			apns2.ReasonUnregistered:

			logger.WithField("id", r.ApnsID).Info("removing subscriber because a relevant error was received from APNS")
			c.Manager().Remove(subscriber)
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
