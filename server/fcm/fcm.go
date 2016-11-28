package fcm

import (
	"fmt"

	"github.com/Bogh/gcm"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/connector"
	"github.com/smancke/guble/server/router"
)

const (
	// schema is the default database schema for FCM
	schema = "fcm_registration"

	deviceTokenKey = "device_token"
	userIDKEy      = "user_id"
)

// Config is used for configuring the Firebase Cloud Messaging component.
type Config struct {
	Enabled              *bool
	APIKey               *string
	Workers              *int
	Endpoint             *string
	Prefix               *string
	AfterMessageDelivery protocol.MessageDeliveryCallback
}

// Connector is the structure for handling the communication with Firebase Cloud Messaging
type fcm struct {
	Config
	connector.Connector
}

// New creates a new *fcm and returns it as an connector.ResponsiveConnector
func New(router router.Router, sender connector.Sender, config Config) (connector.ResponsiveConnector, error) {
	baseConn, err := connector.NewConnector(router, sender, connector.Config{
		Name:       "fcm",
		Schema:     schema,
		Prefix:     *config.Prefix,
		URLPattern: fmt.Sprintf("/{%s}/{%s}/{%s:.*}", deviceTokenKey, userIDKEy, connector.TopicParam),
		Workers:    *config.Workers,
	})
	if err != nil {
		logger.WithError(err).Error("Base connector error")
		return nil, err
	}

	newConn := &fcm{config, baseConn}
	newConn.SetResponseHandler(newConn)
	return newConn, nil
}

func (f *fcm) HandleResponse(request connector.Request, responseIface interface{}, err error) error {
	if err != nil && !isValidResponseError(err) {
		logger.WithField("error", err.Error()).Error("Error sending message to FCM")
		return err
	}
	message := request.Message()
	subscriber := request.Subscriber()

	response, ok := responseIface.(*gcm.Response)
	if !ok {
		return fmt.Errorf("Invalid FCM Response")
	}

	logger.WithField("messageID", message.ID).Debug("Delivered message to FCM")
	subscriber.SetLastID(message.ID)
	if err := f.Manager().Update(request.Subscriber()); err != nil {
		return err
	}
	if response.Ok() {
		return nil
	}

	logger.WithField("success", response.Success).Debug("Handling FCM Error")

	switch errText := response.Error.Error(); errText {
	case "NotRegistered":
		logger.Debug("Removing not registered FCM subscription")
		f.Manager().Remove(subscriber)
		return response.Error
	case "InvalidRegistration":
		logger.WithField("jsonError", errText).Error("InvalidRegistration of FCM subscription")
	default:
		logger.WithField("jsonError", errText).Error("Unexpected error while sending to FCM")
	}

	if response.CanonicalIDs != 0 {
		// we only send to one receiver, so we know that we can replace the old id with the first registration id (=canonical id)
		return f.replaceCanonical(request.Subscriber(), response.Results[0].RegistrationID)
	}
	return nil
}

func (f *fcm) replaceCanonical(subscriber connector.Subscriber, newToken string) error {
	manager := f.Manager()
	err := manager.Remove(subscriber)
	if err != nil {
		return err
	}

	topic := subscriber.Route().Path
	params := subscriber.Route().RouteParams.Copy()

	params[deviceTokenKey] = newToken

	newSubscriber, err := manager.Create(topic, params)
	go f.Run(newSubscriber)
	return err
}
