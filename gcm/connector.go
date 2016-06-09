package gcm

import (
	log "github.com/Sirupsen/logrus"
	"github.com/alexjlockwood/gcm"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"
	"strconv"

	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

const (
	// registrationsSchema is the default sqlite schema for GCM
	schema = "gcm_registration"

	// sendRetries is the number of retries when sending a message
	sendRetries = 5

	// broadcastRetries is the number of retries when broadcasting a message
	broadcastRetries = 3

	subscribePrefixPath = "subscribe"

	// default channel buffer size
	bufferSize = 1000
)

var logger = log.WithField("module", "gcm")

// Connector is the structure for handling the communication with Google Cloud Messaging
type Connector struct {
	Sender        *gcm.Sender
	router        server.Router
	kvStore       store.KVStore
	prefix        string
	pipelineC     chan *pipeMessage
	stopC         chan bool
	nWorkers      int
	wg            sync.WaitGroup
	broadcastPath string
}

// New creates a new *Connector without starting it
func New(router server.Router, prefix string, gcmAPIKey string, nWorkers int) (*Connector, error) {
	kvStore, err := router.KVStore()
	if err != nil {
		return nil, err
	}

	return &Connector{
		Sender:        &gcm.Sender{ApiKey: gcmAPIKey},
		router:        router,
		kvStore:       kvStore,
		prefix:        prefix,
		pipelineC:     make(chan *pipeMessage, bufferSize),
		stopC:         make(chan bool),
		nWorkers:      nWorkers,
		broadcastPath: removeTrailingSlash(prefix) + "/broadcast",
	}, nil
}

// Start opens the connector, creates more goroutines / workers to handle messages coming from the router
func (conn *Connector) Start() error {
	// broadcast route will be a custom subscription
	// broadcastRoute := server.NewRoute(conn.broadcastPath, "gcm_connector", "gcm_connector", bufferSize)
	// conn.router.Subscribe(broadcastRoute)

	// blocking until current subs are loaded
	conn.loadSubs()

	go func() {
		conn.wg.Add(conn.nWorkers)
		for id := 1; id <= conn.nWorkers; id++ {
			go conn.loopPipeline(id)
		}
	}()
	return nil
}

// Stop signals the closing of GCMConnector
func (conn *Connector) Stop() error {
	logger.Debug("Stopping ...")
	close(conn.stopC)
	conn.wg.Wait()
	logger.Debug("Stopped")
	return nil
}

// Check returns nil if health-check succeeds, or an error if health-check fails
// by sending a request with only apikey. If the response is processed by the GCM endpoint
// the gcmStatus will be UP, otherwise the error from sending the message will be returned.
func (conn *Connector) Check() error {
	payload := messageMap(&protocol.Message{Body: []byte(`{"registration_ids":["ABC"]}`)})
	_, err := conn.Sender.Send(gcm.NewMessage(payload, ""), sendRetries)
	if err != nil {
		logger.WithField("err", err).Error("Error sending ping message")
		return err
	}
	return nil
}

// loopPipeline awaits in a loop for messages subscriptions to be forwarded to GCM,
// until the stop-channel is closed
func (conn *Connector) loopPipeline(id int) {
	defer func() {
		logger.WithField("id", id).Debug("Worker stopped")
		conn.wg.Done()
	}()
	logger.WithField("id", id).Debug("Worker started")

	for {
		select {
		case pm := <-conn.pipelineC:
			if pm != nil {
				conn.sendMessage(pm)
			}
		case <-conn.stopC:
			return
		}
	}
}

func (conn *Connector) sendMessage(pm *pipeMessage) {
	gcmID := pm.sub.route.ApplicationID
	payload := pm.payload()

	gcmMessage := gcm.NewMessage(payload, gcmID)
	logger.WithFields(log.Fields{
		"gcm_id":      gcmID,
		"pipe_length": len(conn.pipelineC),
	}).Debug("Sending message")

	result, err := conn.Sender.Send(gcmMessage, sendRetries)
	if err != nil {
		pm.errC <- err
		return
	}
	pm.resultC <- result
}

// func (conn *GCMConnector) broadcastMessage(m *protocol.Message) {
// 	topic := m.Path
// 	payload := messageMap(m)
// 	protocol.Debug("gcm: broadcasting message with topic: %v ; channel length: %v", string(topic), len(conn.pipelineC))

// 	subscriptions := conn.kvStore.Iterate(schema, "")
// 	count := 0
// 	for {
// 		select {
// 		case entry, ok := <-subscriptions:
// 			if !ok {
// 				protocol.Info("gcm: sent message to %v receivers", count)
// 				return
// 			}
// 			gcmID := entry[0]
// 			//TODO collect 1000 gcmIds and send them in one request!
// 			broadcastMessage := gcm.NewMessage(payload, gcmID)
// 			go func() {
// 				//TODO error handling of response!
// 				_, err := conn.Sender.Send(broadcastMessage, broadcastRetries)
// 				protocol.Debug("gcm: sent broadcast message to gcmID=%v", gcmID)
// 				if err != nil {
// 					protocol.Err("gcm: error sending broadcast message to gcmID=%v: %v", gcmID, err.Error())
// 				}
// 			}()
// 			count++
// 		}
// 	}
// }

// GetPrefix is used to satisfy the HTTP handler interface
func (conn *Connector) GetPrefix() string {
	return conn.prefix
}

func (conn *Connector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logger.WithField("method", r.Method).Error("Only HTTP post method supported.")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, gcmID, topic, err := conn.parseParams(r.URL.Path)
	if err != nil {
		http.Error(w, "Invalid Parameters in request", http.StatusBadRequest)
		return
	}
	initSub(conn, topic, userID, gcmID, 0)
	fmt.Fprintf(w, "registered: %v\n", topic)
}

// parseParams will parse the HTTP URL with format /gcm/:userid/:gcmid/subscribe/*topic
// returning the parsed Params, or error if the request is not in the correct format
func (conn *Connector) parseParams(path string) (userID, gcmID, topic string, err error) {
	currentURLPath := removeTrailingSlash(path)

	if strings.HasPrefix(currentURLPath, conn.prefix) != true {
		err = errors.New("gcm: GCM request is not starting with gcm prefix")
		return
	}
	pathAfterPrefix := strings.TrimPrefix(currentURLPath, conn.prefix)

	splitParams := strings.SplitN(pathAfterPrefix, "/", 3)
	if len(splitParams) != 3 {
		err = errors.New("gcm: GCM request has wrong number of params")
		return
	}
	userID = splitParams[0]
	gcmID = splitParams[1]

	if strings.HasPrefix(splitParams[2], subscribePrefixPath+"/") != true {
		err = errors.New("gcm: GCM request third param is not subscribe")
		return
	}
	topic = strings.TrimPrefix(splitParams[2], subscribePrefixPath)
	return userID, gcmID, topic, nil
}

func (conn *Connector) loadSubs() {
	subscriptions := conn.kvStore.Iterate(schema, "")
	count := 0
	for {
		select {
		case entry, ok := <-subscriptions:
			if !ok {
				logger.WithField("count", count).Info("Loaded GCM subscriptions")
				return
			}
			conn.loadSub(entry)
			count++
		}
	}
}

func (conn *Connector) loadSub(entry [2]string) {
	gcmID := entry[0]
	values := strings.Split(entry[1], ":")
	userID := values[0]
	topic := values[1]
	lastID, err := strconv.ParseUint(values[2], 10, 64)
	if err != nil {
		lastID = 0
	}

	initSub(conn, topic, userID, gcmID, lastID)

	logger.WithFields(log.Fields{
		"gcm_id":  gcmID,
		"user_id": userID,
		"topic":   topic,
	}).Debug("Loaded GCM subscription")
}

func removeTrailingSlash(path string) string {
	if len(path) > 1 && path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}
