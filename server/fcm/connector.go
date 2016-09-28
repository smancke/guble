package fcm

import (
	"github.com/Bogh/gcm"

	log "github.com/Sirupsen/logrus"

	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/cluster"
	"github.com/smancke/guble/server/kvstore"
	"github.com/smancke/guble/server/router"

	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/smancke/guble/server/metrics"
)

const (
	// registrationsSchema is the default sqlite schema for GCM
	schema = "gcm_registration"

	// sendRetries is the number of retries when sending a message
	sendRetries = 5

	sendTimeout = time.Second

	subscribePrefixPath = "subscribe"

	// default channel buffer size
	bufferSize = 1000

	syncPath = protocol.Path("/fcm/sync")
)

var logger = log.WithFields(log.Fields{
	"app":    "guble",
	"module": "fcm",
})

// Connector is the structure for handling the communication with Google Firebase Cloud Messaging
type Connector struct {
	Sender        gcm.Sender
	router        router.Router
	cluster       *cluster.Cluster
	kvStore       kvstore.KVStore
	prefix        string
	pipelineC     chan *pipeMessage
	stopC         chan bool
	nWorkers      int
	wg            sync.WaitGroup
	broadcastPath string
	subscriptions map[string]*subscription
}

// New creates a new *Connector without starting it
func New(router router.Router, prefix string, APIKey string, nWorkers int, endpoint string) (*Connector, error) {
	kvStore, err := router.KVStore()
	if err != nil {
		return nil, err
	}

	if endpoint != "" {
		logger.WithField("fcmEndpoint", endpoint).Info("using FCM endpoint")
		gcm.GcmSendEndpoint = endpoint
	}

	return &Connector{
		Sender:        gcm.NewSender(APIKey, sendRetries, sendTimeout),
		router:        router,
		cluster:       router.Cluster(),
		kvStore:       kvStore,
		prefix:        prefix,
		pipelineC:     make(chan *pipeMessage, bufferSize),
		stopC:         make(chan bool),
		nWorkers:      nWorkers,
		broadcastPath: removeTrailingSlash(prefix) + "/broadcast",
		subscriptions: make(map[string]*subscription),
	}, nil
}

// Start opens the connector, creates more goroutines / workers to handle messages coming from the router
func (conn *Connector) Start() error {
	conn.reset()

	startMetrics()

	// start subscription sync loop if we are in cluster mode
	if conn.cluster != nil {
		if err := conn.syncLoop(); err != nil {
			return err
		}
	}

	// blocking until current subs are loaded
	go conn.loadSubscriptions()

	go func() {
		for id := 1; id <= conn.nWorkers; id++ {
			go conn.loopPipeline(id)
		}
	}()
	return nil
}

func (conn *Connector) reset() {
	conn.stopC = make(chan bool)
	conn.subscriptions = make(map[string]*subscription)
}

// Stop signals the closing of FCM Connector
func (conn *Connector) Stop() error {
	logger.Debug("Stopping ...")
	close(conn.stopC)
	conn.wg.Wait()
	logger.Debug("Stopped")
	return nil
}

// Check returns nil if health-check succeeds, or an error if health-check fails
// by sending a request with only apikey. If the response is processed by the FCM endpoint
// the status will be UP, otherwise the error from sending the message will be returned.
func (conn *Connector) check() error {
	message := &gcm.Message{
		To: "ABC",
	}
	_, err := conn.Sender.Send(message)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Error checking FCM connection")
		return err
	}
	return nil
}

// loopPipeline awaits in a loop for messages subscriptions to be forwarded to FCM,
// until the stop-channel is closed
func (conn *Connector) loopPipeline(id int) {
	conn.wg.Add(1)
	defer func() {
		logger.WithField("id", id).Debug("Worker stopped")
		conn.wg.Done()
	}()
	logger.WithField("id", id).Debug("Worker started")

	for {
		select {
		case pm := <-conn.pipelineC:
			// only forward to FCM messages which have subscription on this node and not the other received from cluster
			if pm != nil {
				if conn.cluster != nil && conn.cluster.Config.ID != pm.message.NodeID {
					pm.ignoreMessage()
					continue
				}
				conn.sendMessage(pm)
			}
		case <-conn.stopC:
			return
		}
	}
}

func (conn *Connector) sendMessage(pm *pipeMessage) {
	fcmID := pm.subscription.route.Get(applicationIDKey)

	fcmMessage := pm.fcmMessage()
	fcmMessage.To = fcmID
	logger.WithFields(log.Fields{
		"fcmTo":      fcmMessage.To,
		"fcmID":      fcmID,
		"pipeLength": len(conn.pipelineC),
	}).Debug("Sending message")

	beforeSend := time.Now()
	response, err := conn.Sender.Send(fcmMessage)
	latencyDuration := time.Now().Sub(beforeSend)

	if err != nil && !pm.subscription.isValidResponseError(err) {
		// Even if we receive an error we could still have a valid response
		pm.errC <- err
		mTotalSentMessageErrors.Add(1)
		metrics.AddToMaps(currentTotalErrorsLatenciesKey, int64(latencyDuration), mMinute, mHour, mDay)
		metrics.AddToMaps(currentTotalErrorsKey, 1, mMinute, mHour, mDay)
		return
	}
	mTotalSentMessages.Add(1)
	metrics.AddToMaps(currentTotalMessagesLatenciesKey, int64(latencyDuration), mMinute, mHour, mDay)
	metrics.AddToMaps(currentTotalMessagesKey, 1, mMinute, mHour, mDay)

	pm.resultC <- response
}

// GetPrefix is used to satisfy the HTTP handler interface
func (conn *Connector) GetPrefix() string {
	return conn.prefix
}

// ServeHTTP handles the subscription in FCM
func (conn *Connector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		logger.WithField("method", r.Method).Error("Only HTTP POST and DELETE methods supported.")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, fcmID, topic, err := conn.parseParams(r.URL.Path)
	if err != nil {
		http.Error(w, "Invalid Parameters in request", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		conn.addSubscription(w, topic, userID, fcmID)
	case http.MethodDelete:
		conn.deleteSubscription(w, topic, userID, fcmID)
	}
}

func (conn *Connector) addSubscription(w http.ResponseWriter, topic, userID, fcmID string) {
	s, err := initSubscription(conn, topic, userID, fcmID, 0, true)
	if err == nil {
		// synchronize subscription after storing if cluster exists
		conn.synchronizeSubscription(topic, userID, fcmID, false)
	} else if err == errSubscriptionExists {
		logger.WithField("subscription", s).Error("Subscription already exists")
		fmt.Fprint(w, "subscription already exists")
		return
	}
	fmt.Fprintf(w, "subscribed: %v\n", topic)
}

func (conn *Connector) deleteSubscription(w http.ResponseWriter, topic, userID, fcmID string) {
	subscriptionKey := composeSubscriptionKey(topic, userID, fcmID)

	s, ok := conn.subscriptions[subscriptionKey]
	if !ok {
		logger.WithField("subscriptionKey", subscriptionKey).
			WithField("subscriptions", conn.subscriptions).
			Info("Subscription not found")
		http.Error(w, "subscription not found", http.StatusNotFound)
		return
	}

	conn.synchronizeSubscription(topic, userID, fcmID, true)

	s.remove()
	fmt.Fprintf(w, "unsubscribed: %v\n", topic)
}

// parseParams will parse the HTTP URL with format /fcm/:userid/:fcmid/subscribe/*topic
// returning the parsed Params, or error if the request is not in the correct format
func (conn *Connector) parseParams(path string) (userID, fcmID, topic string, err error) {
	currentURLPath := removeTrailingSlash(path)

	if !strings.HasPrefix(currentURLPath, conn.prefix) {
		err = errors.New("FCM request is not starting with correct prefix")
		return
	}
	pathAfterPrefix := strings.TrimPrefix(currentURLPath, conn.prefix)

	splitParams := strings.SplitN(pathAfterPrefix, "/", 3)
	if len(splitParams) != 3 {
		err = errors.New("FCM request has wrong number of params")
		return
	}
	userID = splitParams[0]
	fcmID = splitParams[1]

	if !strings.HasPrefix(splitParams[2], subscribePrefixPath+"/") {
		err = errors.New("FCM request third param is not subscribe")
		return
	}
	topic = strings.TrimPrefix(splitParams[2], subscribePrefixPath)
	return userID, fcmID, topic, nil
}

func (conn *Connector) loadSubscriptions() {
	count := 0
	for entry := range conn.kvStore.Iterate(schema, "") {
		conn.loadSubscription(entry)
		count++
	}
	logger.WithField("count", count).Info("Loaded FCM subscriptions")
}

// loadSubscription loads a kvstore entry and creates a subscription from it
func (conn *Connector) loadSubscription(entry [2]string) {
	fcmID := entry[0]
	values := strings.Split(entry[1], ":")
	userID := values[0]
	topic := values[1]
	lastID, err := strconv.ParseUint(values[2], 10, 64)
	if err != nil {
		lastID = 0
	}

	initSubscription(conn, topic, userID, fcmID, lastID, false)

	logger.WithFields(log.Fields{
		"fcmID":  fcmID,
		"userID": userID,
		"topic":  topic,
		"lastID": lastID,
	}).Debug("Loaded FCM subscription")
}

// Creates a route and listens for subscription synchronization
func (conn *Connector) syncLoop() error {
	r := router.NewRoute(router.RouteConfig{
		Path:        syncPath,
		ChannelSize: 5000,
	})
	_, err := conn.router.Subscribe(r)
	if err != nil {
		return err
	}

	go func() {
		logger.Info("Sync loop starting")
		conn.wg.Add(1)

		defer func() {
			logger.Info("Sync loop stopped")
			conn.wg.Done()
		}()

		for {
			select {
			case m, opened := <-r.MessagesChannel():
				if !opened {
					logger.Error("Sync loop channel closed")
					return
				}

				if m.NodeID == conn.cluster.Config.ID {
					logger.Debug("Received own subscription loop")
					continue
				}

				subscriptionSync, err := (&subscriptionSync{}).Decode(m.Body)
				if err != nil {
					logger.WithError(err).Error("Error decoding subscription sync")
					continue
				}

				logger.Debug("Initializing sync subscription without storing it.")
				if subscriptionSync.Remove {
					subscriptionKey := composeSubscriptionKey(
						subscriptionSync.Topic,
						subscriptionSync.UserID,
						subscriptionSync.FCMID)
					if s, ok := conn.subscriptions[subscriptionKey]; ok {
						s.remove()
					}
					continue
				}

				if _, err := initSubscription(
					conn,
					subscriptionSync.Topic,
					subscriptionSync.UserID,
					subscriptionSync.FCMID,
					0,
					false); err != nil {
					logger.WithError(err).Error("Error synchronizing subscription")
				}
			case <-conn.stopC:
				return
			}
		}
	}()

	return nil
}

func (conn *Connector) synchronizeSubscription(topic, userID, fcmID string, remove bool) error {
	// there is no cluster setup, no need for synchronization of subscription
	if conn.cluster == nil {
		return nil
	}

	data, err := (&subscriptionSync{topic, userID, fcmID, remove}).Encode()
	if err != nil {
		return err
	}

	return conn.router.HandleMessage(&protocol.Message{
		Path: syncPath,
		Body: data,
	})
}

func removeTrailingSlash(path string) string {
	if len(path) > 1 && path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}

func composeSubscriptionKey(topic, userID, fcmID string) string {
	return fmt.Sprintf("%s %s:%s %s:%s",
		topic,
		applicationIDKey, fcmID,
		userIDKey, userID)
}
