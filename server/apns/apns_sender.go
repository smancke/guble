package apns

import (
	"errors"
	"github.com/sideshow/apns2"
	"github.com/smancke/guble/server/connector"
)

const (
	// deviceIDKey is the key name set on the route params to identify the application
	deviceIDKey = "device_token"
	userIDKey   = "user_id"
)

var (
	errPusherInvalidParams = errors.New("Invalid parameters of APNS Pusher")
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
	logger.WithField("payload", string(request.Message().Body)).
		WithField("devicetoken", request.Subscriber().Route().Get(deviceIDKey)).
		WithField("apptopic", s.appTopic).
		Debug("Trying to push a message to APNS")
	return s.client.Push(&apns2.Notification{
		Priority:    apns2.PriorityHigh,
		Topic:       s.appTopic,
		DeviceToken: request.Subscriber().Route().Get(deviceIDKey),
		Payload:     request.Message().Body,
	})
}
