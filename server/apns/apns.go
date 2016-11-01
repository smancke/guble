package apns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/sideshow/apns2"
	"github.com/smancke/guble/server/connector"
	"github.com/smancke/guble/server/kvstore"
	"github.com/smancke/guble/server/router"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	Workers             *int
}

// Connector is the structure for handling the communication with APNS
type Connector struct {
	queue      connector.Queue
	router     router.Router
	kvStore    kvstore.KVStore
	prefix     string
	subs       map[string]*sub
	context    context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
}

// New creates a new *Connector without starting it
func New(router router.Router, prefix string, config Config) (*Connector, error) {
	kvStore, err := router.KVStore()
	if err != nil {
		log.WithError(err).Error("APNS KVStore error")
		return nil, err
	}
	sender, err := newSender(config)
	if err != nil {
		log.WithError(err).Error("APNS Sender error")
		return nil, err
	}
	newConn := &Connector{
		router:  router,
		kvStore: kvStore,
		prefix:  prefix,
	}
	newConn.queue = connector.NewQueue(sender, newConn, *config.Workers)
	return newConn, nil
}

func (conn *Connector) Start() error {
	conn.reset()

	conn.context, conn.cancelFunc = context.WithCancel(context.Background())

	if conn.queue == nil {
		return errors.New("internal queue should have been already created")
	}
	conn.queue.Start()

	return nil
}

func (conn *Connector) reset() {
	conn.subs = make(map[string]*sub)
}

// Stop the APNS Connector
func (conn *Connector) Stop() error {
	logger.Debug("stopping")
	// first cancel all subs-goroutines
	conn.cancelFunc()
	// then close the queue:
	// - first the requests channel because push() will not be called anymore
	// - then the responses channel, after all the responses are received from the APNS service
	conn.queue.Close()
	logger.Debug("stopped")
	return nil
}

func (conn *Connector) HandleResponse(request connector.Request, responseIface interface{}, errSend error) error {
	log.Debug("HandleResponse")
	if errSend != nil {
		logger.WithError(errSend).Error("APNS error when trying to send notification")
		return errSend
	}
	if rsp, ok := responseIface.(*apns2.Response); ok {
		if !rsp.Sent() {
			log.WithField("id", rsp.ApnsID).WithField("reason", rsp.Reason).Error(errNotSentMsg)
		} else {
			log.WithField("id", rsp.ApnsID).Debug("APNS notification was successfully sent")
		}
		messageID := request.Message().ID
		if err := request.Subscriber().SetLastID(messageID); err != nil {
			//TODO Cosmin Bogdan: error-handling
		}

		//TODO Cosmin Bogdan: extra-APNS-handling
	}
	return nil
}

// GetPrefix is used to satisfy the HTTP handler interface
func (conn *Connector) GetPrefix() string {
	return conn.prefix
}

// ServeHTTP handles the subscription-related processes in APNS
func (conn *Connector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete && r.Method != http.MethodGet {
		logger.WithField("method", r.Method).Error("Only HTTP POST, GET and DELETE methods are supported.")
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	userID, apnsID, unparsedPath, err := conn.parseUserIDAndDeviceID(r.URL.Path)
	if err != nil {
		http.Error(w, `{"error":"invalid parameters in request"}`, http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPost:
		topic, err := conn.parseTopic(unparsedPath)
		if err != nil {
			http.Error(w, `{"error":"invalid parameters in request"}`, http.StatusBadRequest)
			return
		}
		conn.addSubscription(w, topic, userID, apnsID)
	case http.MethodDelete:
		topic, err := conn.parseTopic(unparsedPath)
		if err != nil {
			http.Error(w, `{"error":"invalid parameters in request"}`, http.StatusBadRequest)
			return
		}
		conn.deleteSubscription(w, topic, userID, apnsID)
	case http.MethodGet:
		conn.retrieveSubscription(w, userID, apnsID)
	}
}

func (conn *Connector) retrieveSubscription(w http.ResponseWriter, userID, apnsID string) {
	topics := make([]string, 0)

	for k, v := range conn.subs {
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

func (conn *Connector) addSubscription(w http.ResponseWriter, topic, userID, apnsID string) {
	s, err := initSubscription(conn, topic, userID, apnsID, 0, true)
	if err == nil {
		// synchronize subscription after storing it (if cluster exists)
		conn.synchronizeSubscription(topic, userID, apnsID, false)
	} else if err == errSubscriptionExists {
		logger.WithField("subscription", s).Error("subscription already exists")
		fmt.Fprint(w, `{"error":"subscription already exists"}`)
		return
	}
	fmt.Fprintf(w, `{"subscribed":"%v"}`, topic)
}

func (conn *Connector) deleteSubscription(w http.ResponseWriter, topic, userID, apnsID string) {
	subscriptionKey := composeSubscriptionKey(topic, userID, apnsID)

	s, ok := conn.subs[subscriptionKey]
	if !ok {
		logger.WithFields(log.Fields{
			"subscriptionKey": subscriptionKey,
			"subscriptions":   conn.subs,
		}).Error("subscription not found")
		http.Error(w, `{"error":"subscription not found"}`, http.StatusNotFound)
		return
	}

	conn.synchronizeSubscription(topic, userID, apnsID, true)

	s.remove()
	fmt.Fprintf(w, `{"unsubscribed":"%v"}`, topic)
}

func (conn *Connector) parseUserIDAndDeviceID(path string) (userID, apnsID, unparsedPath string, err error) {
	currentURLPath := removeTrailingSlash(path)

	if !strings.HasPrefix(currentURLPath, conn.prefix) {
		err = errors.New("APNS request is not starting with correct prefix")
		return
	}
	pathAfterPrefix := strings.TrimPrefix(currentURLPath, conn.prefix)

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
func (conn *Connector) parseTopic(unparsedPath string) (topic string, err error) {
	if !strings.HasPrefix(unparsedPath, subscribePrefixPath+"/") {
		err = errors.New("APNS request third param is not subscribe")
		return
	}
	topic = strings.TrimPrefix(unparsedPath, subscribePrefixPath)
	return topic, nil
}

func (conn *Connector) loadSubscriptions() {
	count := 0
	for entry := range conn.kvStore.Iterate(schema, "") {
		conn.loadSubscription(entry)
		count++
	}
	logger.WithField("count", count).Info("loaded all APNS subscriptions")
}

// loadSubscription loads a kvstore entry and creates a subscription from it
func (conn *Connector) loadSubscription(entry [2]string) {
	apnsID := entry[0]
	values := strings.Split(entry[1], ":")
	userID := values[0]
	topic := values[1]
	lastID, err := strconv.ParseUint(values[2], 10, 64)
	if err != nil {
		lastID = 0
	}

	initSubscription(conn, topic, userID, apnsID, lastID, false)

	logger.WithFields(log.Fields{
		"apnsID": apnsID,
		"userID": userID,
		"topic":  topic,
		"lastID": lastID,
	}).Debug("loaded one APNS subscription")
}

// Check returns nil if health-check succeeds, or an error if health-check fails
func (conn *Connector) Check() error {
	return nil
}

func (conn *Connector) synchronizeSubscription(topic, userID, apnsID string, remove bool) error {
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
