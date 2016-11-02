package apns

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/sideshow/apns2"
	"github.com/smancke/guble/server/connector"
	"github.com/smancke/guble/server/router"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

const (
	// schema is the default database schema for APNS
	schema = "apns_registration"

	errNotSentMsg = "APNS notification was not sent"

	subscribePrefixPath = "subscribe"
)

// Config is used for configuring the APNS module.
type Config struct {
	Enabled             *bool
	Production          *bool
	CertificateFileName *string
	CertificateBytes    *[]byte
	CertificatePassword *string
	AppTopic            *string
	Workers             *int
}

// conn is the private struct for handling the communication with APNS
type conn struct {
	connector.Conn
	subs map[string]*sub
}

// New creates a new Connector without starting it
func New(router router.Router, prefix string, config Config) (connector.Connector, error) {
	sender, err := newSender(config)
	if err != nil {
		log.WithError(err).Error("APNS Sender error")
		return nil, err
	}
	connectorConfig := connector.Config{
		Name:       "apns",
		Schema:     schema,
		Prefix:     prefix,
		URLPattern: "/{device_token}/{user_id}/{topic:.*}",
	}
	baseConn, err := connector.NewConnector(router, sender, connectorConfig)
	if err != nil {
		log.WithError(err).Error("Base connector error")
		return nil, err
	}
	newConn := &conn{
		Conn: baseConn,
		subs: make(map[string]*sub),
	}

	// queue is created here since it uses as a param (ResponseHandler) the new connector itself (newConn)
	newConn.Queue = connector.NewQueue(sender, newConn, *config.Workers)
	return newConn, nil
}

func (c conn) HandleResponse(request connector.Request, responseIface interface{}, errSend error) error {
	log.Debug("HandleResponse")
	if errSend != nil {
		logger.WithError(errSend).Error("error when trying to send APNS notification")
		return errSend
	}
	if r, ok := responseIface.(*apns2.Response); ok {
		messageID := request.Message().ID
		if err := request.Subscriber().SetLastID(messageID); err != nil {
			logger.WithError(errSend).Error("error when setting the last-id for the subscriber")
			return err
		}
		if r.Sent() {
			log.WithField("id", r.ApnsID).Debug("APNS notification was successfully sent")
			return nil
		}
		log.WithField("id", r.ApnsID).WithField("reason", r.Reason).Error(errNotSentMsg)
		switch r.Reason {
		case
			apns2.ReasonMissingDeviceToken,
			apns2.ReasonBadDeviceToken,
			apns2.ReasonDeviceTokenNotForTopic,
			apns2.ReasonUnregistered:
			// TODO Cosmin Bogdan remove subscription (+Unsubscribe and stop subscription loop) ?
		}
		//TODO Cosmin Bogdan: extra-APNS-handling
	}
	return nil
}

// ServeHTTP handles the subscription-related processes in APNS.
// It complies with the service.Endpoint interface.
func (c *conn) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete && r.Method != http.MethodGet {
		logger.WithField("method", r.Method).Error("Only HTTP POST, GET and DELETE methods are supported.")
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	userID, apnsID, unparsedPath, err := c.parseUserIDAndDeviceID(r.URL.Path)
	if err != nil {
		http.Error(w, `{"error":"invalid parameters in request"}`, http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPost:
		topic, err := c.parseTopic(unparsedPath)
		if err != nil {
			http.Error(w, `{"error":"invalid parameters in request"}`, http.StatusBadRequest)
			return
		}
		c.addSubscription(w, topic, userID, apnsID)
	case http.MethodDelete:
		topic, err := c.parseTopic(unparsedPath)
		if err != nil {
			http.Error(w, `{"error":"invalid parameters in request"}`, http.StatusBadRequest)
			return
		}
		c.deleteSubscription(w, topic, userID, apnsID)
	case http.MethodGet:
		c.retrieveSubscription(w, userID, apnsID)
	}
}

func (c *conn) retrieveSubscription(w http.ResponseWriter, userID, apnsID string) {
	topics := make([]string, 0)

	for k, v := range c.subs {
		logger.WithField("key", k).Debug("retrieveSubscription")
		if v.route.Get(applicationIDKey) == apnsID && v.route.Get(userIDKey) == userID {
			logger.WithField("path", v.route.Path).Debug("retrieveSubscription path")
			topics = append(topics, strings.TrimPrefix(string(v.route.Path), "/"))
		}
	}

	sort.Strings(topics)
	err := json.NewEncoder(w).Encode(topics)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
	}
}

func (c *conn) addSubscription(w http.ResponseWriter, topic, userID, apnsID string) {
	s, err := initSubscription(c, topic, userID, apnsID, 0, true)
	if err == nil {
		// synchronize subscription after storing it (if cluster exists)
		c.synchronizeSubscription(topic, userID, apnsID, false)
	} else if err == errSubscriptionExists {
		logger.WithField("subscription", s).Error("subscription already exists")
		fmt.Fprint(w, `{"error":"subscription already exists"}`)
		return
	}
	fmt.Fprintf(w, `{"subscribed":"%v"}`, topic)
}

func (c *conn) deleteSubscription(w http.ResponseWriter, topic, userID, apnsID string) {
	subscriptionKey := composeSubscriptionKey(topic, userID, apnsID)

	s, ok := c.subs[subscriptionKey]
	if !ok {
		logger.WithFields(log.Fields{
			"subscriptionKey": subscriptionKey,
			"subscriptions":   c.subs,
		}).Error("subscription not found")
		http.Error(w, `{"error":"subscription not found"}`, http.StatusNotFound)
		return
	}

	c.synchronizeSubscription(topic, userID, apnsID, true)

	s.remove()
	fmt.Fprintf(w, `{"unsubscribed":"%v"}`, topic)
}

func (c *conn) parseUserIDAndDeviceID(path string) (userID, apnsID, unparsedPath string, err error) {
	currentURLPath := removeTrailingSlash(path)

	if !strings.HasPrefix(currentURLPath, c.Config.Prefix) {
		err = errors.New("APNS request is not starting with correct prefix")
		return
	}
	pathAfterPrefix := strings.TrimPrefix(currentURLPath, c.Config.Prefix)

	splitParams := strings.SplitN(pathAfterPrefix, "/", 3)
	if len(splitParams) != 3 {
		err = errors.New("APNS request has wrong number of params")
		return
	}
	userID = splitParams[0]
	apnsID = splitParams[1]
	unparsedPath = splitParams[2]
	return
}

// parseTopic will parse the HTTP URL with format /apns/:userid/:apnsid/subscribe/*topic
// returning the parsed Params, or error if the request is not in the correct format
func (c *conn) parseTopic(unparsedPath string) (topic string, err error) {
	if !strings.HasPrefix(unparsedPath, subscribePrefixPath+"/") {
		err = errors.New("APNS request third param is not subscribe")
		return
	}
	topic = strings.TrimPrefix(unparsedPath, subscribePrefixPath)
	return topic, nil
}

func (c *conn) loadSubscriptions() {
	count := 0
	for entry := range c.KVStore.Iterate(schema, "") {
		c.loadSubscription(entry)
		count++
	}
	logger.WithField("count", count).Info("loaded all APNS subscriptions")
}

// loadSubscription loads a kvstore entry and creates a subscription from it
func (c *conn) loadSubscription(entry [2]string) {
	apnsID := entry[0]
	values := strings.Split(entry[1], ":")
	userID := values[0]
	topic := values[1]
	lastID, err := strconv.ParseUint(values[2], 10, 64)
	if err != nil {
		lastID = 0
	}

	initSubscription(c, topic, userID, apnsID, lastID, false)

	logger.WithFields(log.Fields{
		"apnsID": apnsID,
		"userID": userID,
		"topic":  topic,
		"lastID": lastID,
	}).Debug("loaded one APNS subscription")
}

// Check returns nil if health-check succeeds, or an error if health-check fails
func (c *conn) Check() error {
	return nil
}

func (c *conn) synchronizeSubscription(topic, userID, apnsID string, remove bool) error {
	//TODO implement
	return nil
}

func removeTrailingSlash(path string) string {
	if len(path) > 1 && path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}

func composeSubscriptionKey(topic, userID, apnsID string) string {
	return fmt.Sprintf("%s %s:%s %s:%s",
		topic,
		applicationIDKey, apnsID,
		userIDKey, userID)
}
