package apns

import (
	"crypto/tls"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"github.com/smancke/guble/server/connector"
	"strings"
)

type Pusher interface {
	Push(*apns2.Notification) (*apns2.Response, error)
}

type sender struct {
	client   Pusher
	appTopic string
}

func newSender(config Config) (*sender, error) {
	client, err := newClient(config)
	if err != nil {
		return nil, err
	}
	return &sender{
		client:   client,
		appTopic: *config.AppTopic,
	}, nil
}

func newClient(c Config) (Pusher, error) {
	var (
		cert    tls.Certificate
		errCert error
	)
	if c.CertificateFileName != nil && *c.CertificateFileName != "" {
		cert, errCert = certificate.FromP12File(*c.CertificateFileName, *c.CertificatePassword)
	} else {
		cert, errCert = certificate.FromP12Bytes(*c.CertificateBytes, *c.CertificatePassword)
	}
	if errCert != nil {
		return nil, errCert
	}
	if *c.Production {
		return apns2.NewClient(cert).Production(), nil
	}
	return apns2.NewClient(cert).Development(), nil
}

func (s sender) Send(request connector.Request) (interface{}, error) {
	r := request.Subscriber().Route()

	//TODO Cosmin: Samsa should generate the Payload or the whole Notification, and JSON-serialize it into the guble-message Body.

	//m := request.Message()
	//n := &apns2.Notification{
	//	Priority:    apns2.PriorityHigh,
	//	Topic:       strings.TrimPrefix(string(s.route.Path), "/"),
	//	DeviceToken: s.route.Get(applicationIDKey),
	//	Payload:     m.Body,
	//}

	topic := strings.TrimPrefix(string(r.Path), "/")
	n := &apns2.Notification{
		Priority:    apns2.PriorityHigh,
		Topic:       s.appTopic,
		DeviceToken: r.Get(applicationIDKey),
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
