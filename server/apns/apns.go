package apns

import (
	"errors"
	"fmt"
	"github.com/cosminrentea/gobbler/server/connector"
	"github.com/cosminrentea/gobbler/server/metrics"
	"github.com/cosminrentea/gobbler/server/router"
	"github.com/sideshow/apns2"
	"time"
)

const (
	// schema is the default database schema for APNS
	schema = "apns_registration"
)

var (
	errSenderNotRecreated = errors.New("APNS Sender could not be recreated.")
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
	IntervalMetrics     *bool
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
	err := a.Connector.Start()
	if err == nil {
		a.startMetrics()
	}
	return err
}

func (a *apns) startMetrics() {
	mTotalSentMessages.Set(0)
	mTotalSendErrors.Set(0)
	mTotalResponseErrors.Set(0)
	mTotalResponseInternalErrors.Set(0)
	mTotalResponseRegistrationErrors.Set(0)
	mTotalResponseOtherErrors.Set(0)
	mTotalSendNetworkErrors.Set(0)
	mTotalSendRetryCloseTLS.Set(0)
	mTotalSendRetryUnrecoverable.Set(0)

	if *a.IntervalMetrics {
		a.startIntervalMetric(mMinute, time.Minute)
		a.startIntervalMetric(mHour, time.Hour)
		a.startIntervalMetric(mDay, time.Hour*24)
	}
}

func (a *apns) startIntervalMetric(m metrics.Map, td time.Duration) {
	metrics.RegisterInterval(a.Context(), m, td, resetIntervalMetrics, processAndResetIntervalMetrics)
}

func (a *apns) HandleResponse(request connector.Request, responseIface interface{}, metadata *connector.Metadata, errSend error) error {
	logger.Info("Handle APNS response")
	if errSend != nil {
		logger.WithField("error", errSend.Error()).WithField("error_type", errSend).Error("error when trying to send APNS notification")
		mTotalSendErrors.Add(1)
		pSendErrors.Inc()
		if *a.IntervalMetrics && metadata != nil {
			addToLatenciesAndCountsMaps(currentTotalErrorsLatenciesKey, currentTotalErrorsKey, metadata.Latency)
		}
		return errSend
	}
	r, ok := responseIface.(*apns2.Response)
	if !ok {
		mTotalResponseErrors.Add(1)
		pResponseErrors.Inc()
		return fmt.Errorf("Response could not be converted to an APNS Response")
	}
	messageID := request.Message().ID
	subscriber := request.Subscriber()
	subscriber.SetLastID(messageID)
	if err := a.Manager().Update(subscriber); err != nil {
		logger.WithField("error", err.Error()).Error("Manager could not update subscription")
		mTotalResponseInternalErrors.Add(1)
		pResponseInternalErrors.Inc()
		return err
	}
	if r.Sent() {
		logger.WithField("id", r.ApnsID).Info("APNS notification was successfully sent")
		mTotalSentMessages.Add(1)
		pSentMessages.Inc()
		if *a.IntervalMetrics && metadata != nil {
			addToLatenciesAndCountsMaps(currentTotalMessagesLatenciesKey, currentTotalMessagesKey, metadata.Latency)
		}
		return nil
	}
	logger.Error("APNS notification was not sent")
	logger.WithField("id", r.ApnsID).WithField("reason", r.Reason).Info("APNS notification was not sent - details")
	switch r.Reason {
	case
		apns2.ReasonMissingDeviceToken,
		apns2.ReasonBadDeviceToken,
		apns2.ReasonDeviceTokenNotForTopic,
		apns2.ReasonUnregistered:

		logger.WithField("id", r.ApnsID).Info("trying to remove subscriber because a relevant error was received from APNS")
		mTotalResponseRegistrationErrors.Add(1)
		pResponseRegistrationErrors.Inc()
		err := a.Manager().Remove(subscriber)
		if err != nil {
			logger.WithField("id", r.ApnsID).Error("could not remove subscriber")
		}
	default:
		logger.Error("handling other APNS errors")
		mTotalResponseOtherErrors.Add(1)
		pResponseOtherErrors.Inc()
	}
	return nil
}
