package apns

import (
	"crypto/tls"
	"errors"
	"net/http"
	"os"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"github.com/smancke/guble/server/router"
)

const (
	// schema is the default database schema for APNS
	schema = "apns_registration"

	msgNotSent = "APNS notification was not sent"
)

var (
	ErrAPNSNotSent = errors.New(msgNotSent)
)

// Config is used for configuring the APNS module.
type Config struct {
	Enabled             *bool
	Production          *bool
	CertificateFileName *string
	CertificateBytes    *[]byte
	CertificatePassword *string
}

// Connector is the structure for handling the communication with APNS
type Connector struct {
	client *apns2.Client
	router router.Router
	prefix string
	stopC  chan bool
	wg     sync.WaitGroup
}

// New creates a new *Connector without starting it
func New(router router.Router, prefix string, config Config) (*Connector, error) {
	return &Connector{
		client: getClient(config),
		router: router,
		prefix: prefix,
		stopC:  make(chan bool),
	}, nil
}

func (conn *Connector) Start() error {
	conn.reset()

	// temporarily: send a notification when connector is starting
	// topic + device are given using environment variables
	if conn.client != nil {
		conn.sendAlert(&apns2.Notification{
			Priority:    apns2.PriorityHigh,
			Topic:       os.Getenv("APNS_TOPIC"),
			DeviceToken: os.Getenv("APNS_DEVICE_TOKEN"),
			Payload: payload.NewPayload().
				AlertTitle("REWE").
				AlertBody("Guble APNS connector just started").
				Badge(1).
				ContentAvailable(),
		})
	}
	return nil
}

func (conn *Connector) reset() {
	conn.stopC = make(chan bool)
}

// Stop the APNS Connector
func (conn *Connector) Stop() error {
	logger.Debug("stopping")
	close(conn.stopC)
	conn.wg.Wait()
	logger.Debug("stopped")
	return nil
}

// GetPrefix is used to satisfy the HTTP handler interface
func (conn *Connector) GetPrefix() string {
	return conn.prefix
}

// ServeHTTP handles the subscription-related processes in APNS
func (conn *Connector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}

// Check returns nil if health-check succeeds, or an error if health-check fails
func (conn *Connector) Check() error {
	return nil
}

func getClient(c Config) *apns2.Client {
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
		log.WithError(errCert).Error("APNS certificate error")
		return nil
	}
	if *c.Production {
		return apns2.NewClient(cert).Production()
	}
	return apns2.NewClient(cert).Development()
}

func (conn *Connector) sendAlert(n *apns2.Notification) error {
	response, errPush := conn.client.Push(n)
	if errPush != nil {
		log.WithError(errPush).Error("APNS error when trying to push notification")
		return errPush
	}
	if !response.Sent() {
		log.WithField("id", response.ApnsID).WithField("reason", response.Reason).Error(msgNotSent)
		return ErrAPNSNotSent
	}
	log.WithField("id", response.ApnsID).Debug("APNS notification was successfully sent")
	return nil
}
