package apns

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"github.com/smancke/guble/server/kvstore"
	"github.com/smancke/guble/server/router"
	"net/http"
	"os"
	"sync"
)

const (
	// schema is the default database schema for APNS
	schema = "apns_registration"

	msgAPNSNotSent = "APNS notification was not sent"
)

var (
	errAPNSNotSent = errors.New(msgAPNSNotSent)
)

// Config is used for configuring the APNS module.
type Config struct {
	Enabled             *bool
	CertificateFileName *string
	CertificatePassword *string
}

// Connector is the structure for handling the communication with APNS
type Connector struct {
	client  *apns2.Client
	router  router.Router
	kvStore kvstore.KVStore
	prefix  string
	stopC   chan bool
	wg      sync.WaitGroup
}

// New creates a new *Connector without starting it
func New(router router.Router, prefix string, config Config) (*Connector, error) {
	kvStore, err := router.KVStore()
	if err != nil {
		return nil, err
	}
	return &Connector{
		client:  getClient(*config.CertificateFileName, *config.CertificatePassword, false),
		router:  router,
		kvStore: kvStore,
		prefix:  prefix,
		stopC:   make(chan bool),
	}, nil
}

func (conn *Connector) Start() error {
	conn.reset()

	// temporarily: send a notification when connector is starting
	// topic + device are given using environment variables
	if conn.client != nil {
		topic := os.Getenv("APNS_TOPIC")
		deviceToken := os.Getenv("APNS_DEVICE_TOKEN")
		p := payload.NewPayload().
			AlertTitle("REWE Guble").
			AlertBody("Guble APNS connector just started").
			ZeroBadge().
			ContentAvailable()
		sendAlert(conn.client, topic, deviceToken, p)
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

// ServeHTTP handles the subscription in APNS
func (conn *Connector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}

// Check returns nil if health-check succeeds, or an error if health-check fails
func (conn *Connector) Check() error {
	return nil
}

func getClient(certFileName string, certPassword string, production bool) *apns2.Client {
	cert, errCert := certificate.FromP12File(certFileName, certPassword)
	if errCert != nil {
		log.WithError(errCert).Error("APNS certificate error")
	}
	if production {
		return apns2.NewClient(cert).Production()
	}
	return apns2.NewClient(cert).Development()
}

func sendAlert(cl *apns2.Client, topic string, deviceToken string, p *payload.Payload) error {
	notification := &apns2.Notification{
		Priority:    apns2.PriorityHigh,
		Topic:       topic,
		DeviceToken: deviceToken,
		Payload:     p,
	}
	response, errPush := cl.Push(notification)
	if errPush != nil {
		log.WithError(errPush).Error("APNS error when trying to push notification")
		return errPush
	}
	if !response.Sent() {
		log.WithField("id", response.ApnsID).WithField("reason", response.Reason).Error(msgAPNSNotSent)
		return errAPNSNotSent
	}
	log.WithField("id", response.ApnsID).Debug("APNS notification was successfully sent")
	return nil
}
