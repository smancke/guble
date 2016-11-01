package apns

import (
	"crypto/tls"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"github.com/smancke/guble/server/connector"
	"strings"
)

type Sender struct {
	client *apns2.Client
}

func newSender(c Config) (*Sender, error) {
	client, err := newClient(c)
	if err != nil {
		return nil, err
	}
	return &Sender{client: client}, nil
}

func (s Sender) Send(request connector.Request) (interface{}, error) {
	r := request.Subscriber().Route()

	//TODO Cosmin: Samsa should generate the Payload or the whole Notification, and JSON-serialize it into the guble-message Body.

	//m := request.Message()
	//n := &apns2.Notification{
	//	Priority:    apns2.PriorityHigh,
	//	Topic:       strings.TrimPrefix(string(s.route.Path), "/"),
	//	DeviceToken: s.route.Get(applicationIDKey),
	//	Payload:     m.Body,
	//}

	n := &apns2.Notification{
		Priority:    apns2.PriorityHigh,
		Topic:       strings.TrimPrefix(string(r.Path), "/"),
		DeviceToken: r.Get(applicationIDKey),
		Payload: payload.NewPayload().
			AlertTitle("Title").
			AlertBody("Text").
			Badge(1).
			ContentAvailable(),
	}
	logger.Debug("Trying to push a message to APNS")
	return s.client.Push(n)
}

func newClient(c Config) (*apns2.Client, error) {
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
