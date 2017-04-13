package apns

import (
	"errors"
	"github.com/cosminrentea/gobbler/server/connector"
	"github.com/jpillora/backoff"
	"github.com/sideshow/apns2"
	"net"
	"time"
)

const (
	// deviceIDKey is the key name set on the route params to identify the application
	deviceIDKey = "device_token"
	userIDKey   = "user_id"
)

var (
	errPusherInvalidParams = errors.New("Invalid parameters of APNS Pusher")
	ErrRetryFailed         = errors.New("Retry failed")
)

type sender struct {
	client   Pusher
	appTopic string
}

func NewSender(config Config) (connector.Sender, error) {
	pusher, err := newPusher(config)
	if err != nil {
		logger.WithField("error", err.Error()).Error("APNS Pusher creation error")
		return nil, err
	}
	return NewSenderUsingPusher(pusher, *config.AppTopic)
}

func NewSenderUsingPusher(pusher Pusher, appTopic string) (connector.Sender, error) {
	if pusher == nil || appTopic == "" {
		return nil, errPusherInvalidParams
	}
	return &sender{
		client:   pusher,
		appTopic: appTopic,
	}, nil
}

func (s sender) Send(request connector.Request) (interface{}, error) {
	l := logger.WithField("correlation_id", request.Message().CorrelationID())
	deviceToken := request.Subscriber().Route().Get(deviceIDKey)
	l.WithField("deviceToken", deviceToken).Info("Trying to push a message to APNS")
	push := func() (interface{}, error) {
		return s.client.Push(&apns2.Notification{
			Priority:    apns2.PriorityHigh,
			Topic:       s.appTopic,
			DeviceToken: deviceToken,
			Payload:     request.Message().Body,
		})
	}
	withRetry := &retryable{
		Backoff: backoff.Backoff{
			Min:    1 * time.Second,
			Max:    10 * time.Second,
			Factor: 2,
			Jitter: true,
		},
		maxTries: 3,
	}
	result, err := withRetry.execute(push)
	if err != nil && err == ErrRetryFailed {
		if closable, ok := s.client.(closable); ok {
			l.Warn("Close TLS and retry again")
			mTotalSendRetryCloseTLS.Add(1)
			pSendRetryCloseTLS.Inc()
			closable.CloseTLS()
			return push()
		} else {
			mTotalSendRetryUnrecoverable.Add(1)
			pSendRetryUnrecoverable.Inc()
			l.Error("Cannot Close TLS. Unrecoverable state")
		}
	}
	return result, err
}

type retryable struct {
	backoff.Backoff
	maxTries int
}

func (r *retryable) execute(op func() (interface{}, error)) (interface{}, error) {
	tryCounter := 0
	for {
		tryCounter++
		result, opError := op()
		// retry on network errors
		if _, ok := opError.(net.Error); ok {
			mTotalSendNetworkErrors.Add(1)
			pSendNetworkErrors.Inc()
			if tryCounter >= r.maxTries {
				return "", ErrRetryFailed
			}
			d := r.Duration()
			logger.WithField("error", opError.Error()).Warn("Retry in ", d)
			time.Sleep(d)
			continue
		} else {
			return result, opError
		}
	}
}
