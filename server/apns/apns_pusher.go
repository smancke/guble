package apns

import (
	"crypto/tls"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"golang.org/x/net/http2"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	//see https://github.com/sideshow/apns2/issues/24 and https://github.com/sideshow/apns2/issues/20
	tlsDialTimeout    = 20 * time.Second
	httpClientTimeout = 30 * time.Second
)

type Pusher interface {
	Push(*apns2.Notification) (*apns2.Response, error)
}

type closable interface {
	CloseTLS()
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

	var clientFactory func(certificate tls.Certificate) *apns2Client
	if *c.Production {
		clientFactory = newProductionClient
	} else {
		clientFactory = newDevelopmentClient
	}

	apns2.TLSDialTimeout = tlsDialTimeout
	apns2.HTTPClientTimeout = httpClientTimeout

	return clientFactory(cert), nil
}

func newProductionClient(certificate tls.Certificate) *apns2Client {
	logger.Info("APNS Pusher in Production mode")
	c := newApns2Client(certificate)
	c.Production()
	return c
}

func newDevelopmentClient(certificate tls.Certificate) *apns2Client {
	logger.Info("APNS Pusher in Development mode")
	c := newApns2Client(certificate)
	c.Development()
	return c
}

type apns2Client struct {
	*apns2.Client

	tlsConn net.Conn
	mu      sync.Mutex
}

func newApns2Client(certificate tls.Certificate) *apns2Client {
	c := &apns2Client{}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
	}
	if len(certificate.Certificate) > 0 {
		tlsConfig.BuildNameToCertificate()
	}
	transport := &http2.Transport{
		TLSClientConfig: tlsConfig,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			conn, err := tls.DialWithDialer(&net.Dialer{Timeout: tlsDialTimeout, KeepAlive: 2 * time.Second}, network, addr, cfg)

			c.mu.Lock()
			defer c.mu.Unlock()
			if err == nil {
				c.tlsConn = conn
			} else {
				c.tlsConn = nil
			}
			return conn, err
		},
	}
	client := &apns2.Client{
		HTTPClient: &http.Client{
			Transport: transport,
			Timeout:   httpClientTimeout,
		},
		Certificate: certificate,
		Host:        apns2.DefaultHost,
	}
	c.Client = client
	return c
}
// interface closable used used by apns_sender
func (c *apns2Client) CloseTLS() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tlsConn != nil {
		c.tlsConn.Close()
		c.tlsConn = nil
	}
}
