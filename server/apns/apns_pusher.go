package apns

import (
	"crypto/tls"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"time"
	"errors"
)

const (
	useClientManager bool = false
)

var (
	ErrNewClientCreation = errors.New("ClientManager cannot be create new APSN2 client")
)

type Pusher interface {
	Push(*apns2.Notification) (*apns2.Response, error)
}

func newPusher(c Config) (Pusher, error) {
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

	var clientFactory func(certificate tls.Certificate) *apns2.Client;
	if *c.Production {
		clientFactory = newProductionClient
	} else {
		clientFactory = newDevelopmentClient
	}

	//see https://github.com/sideshow/apns2/issues/24 and https://github.com/sideshow/apns2/issues/20
	apns2.TLSDialTimeout = 40 * time.Second
	apns2.HTTPClientTimeout = 60 * time.Second

	if useClientManager {
		clientManager := apns2.NewClientManager()
		clientManager.MaxSize = 4
		clientManager.MaxAge = 1 * time.Minute
		clientManager.Factory = clientFactory
		mp := multiConnectionPusher {
			cert: cert,
			clientManager: clientManager,
		}
		return mp, nil

	} else {
		return clientFactory(cert), nil
	}
}

type multiConnectionPusher struct {
	cert tls.Certificate
	clientManager *apns2.ClientManager
}

func (mcp multiConnectionPusher) Push(n *apns2.Notification) (*apns2.Response, error) {
	client := mcp.clientManager.Get(mcp.cert)
	if client == nil {
		return nil, ErrNewClientCreation
	}
	return client.Push(n)
}

func newProductionClient(certificate tls.Certificate) *apns2.Client {
	logger.Info("APNS Pusher in Production mode")
	return apns2.NewClient(certificate).Production()
}
func newDevelopmentClient(certificate tls.Certificate) *apns2.Client {
	logger.Info("APNS Pusher in Development mode")
	return apns2.NewClient(certificate).Development()
}
