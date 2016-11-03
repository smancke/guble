package apns

import (
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	"github.com/smancke/guble/server/connector"
	"strings"
)

const (
	// deviceIDKey is the key name set on the route params to identify the application
	deviceIDKey = "device_id"
)

type sender struct {
	client   Pusher
	appTopic string
}

func newSender(pusher Pusher, config Config) (connector.Sender, error) {
	return &sender{
		client:   pusher,
		appTopic: *config.AppTopic,
	}, nil
}

func (s sender) Send(request connector.Request) (interface{}, error) {
	route := request.Subscriber().Route()

	//TODO Cosmin: Samsa should generate the Payload or the whole Notification, and JSON-serialize it into the guble-message Body.

	//m := request.Message()
	//n := &apns2.Notification{
	//	Priority:    apns2.PriorityHigh,
	//	Topic:       strings.TrimPrefix(string(s.route.Path), "/"),
	//	DeviceToken: s.route.Get(applicationIDKey),
	//	Payload:     m.Body,
	//}

	topic := strings.TrimPrefix(string(route.Path), "/")
	n := &apns2.Notification{
		Priority:    apns2.PriorityHigh,
		Topic:       s.appTopic,
		DeviceToken: route.Get(deviceIDKey),
		Payload: payload.NewPayload().
			AlertTitle("Title").
			AlertBody("Body").
			Custom("topic", topic).
			Badge(1).
			ContentAvailable(),
	}
	logger.Debug("Trying to push a message to APNS")
	return s.client.Push(n)
}
