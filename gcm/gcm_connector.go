package gcm

import (
	log "github.com/Sirupsen/logrus"
	"github.com/alexjlockwood/gcm"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server"
	"github.com/smancke/guble/store"

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

// GCMConnector is the structure for handling the communication with Google Cloud Messaging
type GCMConnector struct {
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

// NewGCMConnector creates a new GCMConnector without starting it
func NewGCMConnector(router server.Router, prefix string, gcmAPIKey string, nWorkers int) (*GCMConnector, error) {
	kvStore, err := router.KVStore()
	if err != nil {
		return nil, err
	}

	return &GCMConnector{
		Sender:        &gcm.Sender{ApiKey: gcmAPIKey},
		router:        router,
		kvStore:       kvStore,
		prefix:        prefix,
		pipelineC:     make(chan *pipeMessage, bufferSize),
		stopC:         make(chan bool),
		nWorkers:      nWorkers,
		broadcastPath: removeTrailingSlash(prefix) + "/broadcast",
		subs:          make(map[string]*sub),
	}, nil
}

// Start opens the connector, creates more goroutines / workers to handle messages coming from the router
func (conn *GCMConnector) Start() error {
	// broadcast route will be a custom subscription
	// broadcastRoute := server.NewRoute(conn.broadcastPath, "gcm_connector", "gcm_connector", bufferSize)
	// conn.router.Subscribe(broadcastRoute)

	go func() {
		//TODO Cosmin: should loadSubscriptions() be taken out of this goroutine, and executed before ?
		// (even if startup-time is longer, the routes are guaranteed to be there right after Start() returns)
		conn.loadSubs()

		conn.wg.Add(conn.nWorkers)
		for id := 1; id <= conn.nWorkers; id++ {
			go conn.loopPipeline(id)
		}
	}()
	return nil
}

// Stop signals the closing of GCMConnector
func (conn *GCMConnector) Stop() error {
	protocol.Debug("gcm: stopping")
	close(conn.stopC)
	conn.wg.Wait()
	protocol.Debug("gcm: stopped")
	return nil
}

// Check returns nil if health-check succeeds, or an error if health-check fails
// by sending a request with only apikey. If the response is processed by the GCM endpoint
// the gcmStatus will be UP, otherwise the error from sending the message will be returned.
func (conn *GCMConnector) Check() error {
	payload := parseMessageToMap(&protocol.Message{Body: []byte(`{"registration_ids":["ABC"]}`)})
	_, err := conn.Sender.Send(gcm.NewMessage(payload, ""), sendRetries)
	if err != nil {
		protocol.Err("gcm: error sending ping message %v", err.Error())
		return err
	}
	return nil
}

// loopPipeline awaits in a loop for messages subscriptions to be forwarded to GCM,
// until the stop-channel is closed
func (conn *GCMConnector) loopPipeline(id int) {
	defer conn.wg.Done()
	logger.WithField("id", id).Debug("starting worker")

	for {
		select {
		case pm, opened := <-conn.pipelineC:
			conn.sendMessage(pm)
		case <-conn.stopC:
			logger.WithFields("id", id).Debug("Stopping worker")
			return
		}
	}
}

func (conn *GCMConnector) sendMessage(pm *pipeMessage) {
	gcmID := pm.route.ApplicationID
	payload := pm.payload()

	gcmMessage := gcm.NewMessage(payload, gcmID)
	protocol.Debug("gcm: sending message to: %v ; channel length: ", gcmID, len(conn.pipelineC))

	result, err := conn.Sender.Send(gcmMessage, sendRetries)
	if err != nil {
		pm.errC <- err
		return
	}
	pm.resultC <- result

}

// func (conn *GCMConnector) broadcastMessage(m *protocol.Message) {
// 	topic := m.Path
// 	payload := parseMessageToMap(m)
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
func (conn *GCMConnector) GetPrefix() string {
	return conn.prefix
}

func (conn *GCMConnector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logger.WithField("method", r.Method).Error("Only HTTP post method supported.")
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
		return
	}

	userID, gcmID, topic, err := conn.parseParams(r.URL.Path)
	if err != nil {
		http.Error(w, "Invalid Parameters in request", http.StatusBadRequest)
		return
	}
	newSub(conn, topic, userID, gcmID)
	fmt.Fprintf(w, "registered: %v\n", topic)
}

// parseParams will parse the HTTP URL with format /gcm/:userid/:gcmid/subscribe/*topic
// returning the parsed Params, or error if the request is not in the correct format
func (conn *GCMConnector) parseParams(path string) (userID, gcmID, topic string, err error) {
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

func (conn *GCMConnector) loadSubs() {
	subscriptions := conn.kvStore.Iterate(schema, "")
	count := 0
	for {
		select {
		case entry, ok := <-subscriptions:
			if !ok {
				logger.WithField("count", count).Info("")
				protocol.Info("gcm: renewed %v GCM subscriptions", count)
				return
			}
			conn.loadSub(entry)
			route := server.NewRoute(topic, gcmID, userID, conn.pipelineC)
			conn.router.Subscribe(route)
			count++
		}
	}
}

func (conn *GCMConnector) loadSub(entry [2]string) {
	gcmID := entry[0]
	values := strings.Split(entry[1], ":")
	userID := values[0]
	topic := values[1]
	lastID := values[2]

	newSub(conn, topic, userID, gcmID)

	logger.WithFields(log.Fields{
		"gcmID":  gcmID,
		"userID": userID,
		"topic":  topic,
	}).Debug("loaded GCM subscription")
}

func removeTrailingSlash(path string) string {
	if len(path) > 1 && path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}
