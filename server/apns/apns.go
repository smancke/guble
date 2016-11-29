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

// apns is the private struct for handling the communication with APNS
type apns struct {
	Config
	connector.Connector
}

// New creates a new connector.ResponsiveConnector without starting it
func New(router router.Router, sender connector.Sender, config Config) (connector.ResponsiveConnector, error) {
	baseConn, err := connector.NewConnector(
		router,
		sender,
		connector.Config{
			Name:       "apns",
			Schema:     schema,
			Prefix:     *config.Prefix,
			URLPattern: fmt.Sprintf("/{%s}/{%s}/{%s:.*}", deviceIDKey, userIDKey, connector.TopicParam),
			Workers:    *config.Workers,
		},
	)
	if err != nil {
		logger.WithError(err).Error("Base connector error")
		return nil, err
	}
	a := &apns{
		Config:    config,
		Connector: baseConn,
	}
	a.SetResponseHandler(a)
	return a, nil
}

func (a *apns) Start() error {
	a.Connector.Start()
	a.startMetrics()
	return nil
}

func (a *apns) startMetrics() {
	//TODO implement APNS metrics (it is a separate task)
}

func (c *apns) HandleResponse(request connector.Request, responseIface interface{}, metadata *connector.Metadata, errSend error) error {
	logger.Debug("HandleResponse")
	if errSend != nil {
		logger.WithError(errSend).Error("error when trying to send APNS notification")
		return errSend
	}
	if r, ok := responseIface.(*apns2.Response); ok {
		messageID := request.Message().ID
		subscriber := request.Subscriber()
		subscriber.SetLastID(messageID)
		if err := c.Manager().Update(subscriber); err != nil {
			logger.WithField("error", err.Error()).Error("Manager could not update subscription")
			return err
		}
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
