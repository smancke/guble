package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	log "github.com/Sirupsen/logrus"

	"github.com/gorilla/mux"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/kvstore"
	"github.com/smancke/guble/server/router"
	"github.com/smancke/guble/server/service"
)

var (
	ErrInternalQueue = errors.New("internal queue should have been already created")
	TopicParam       = "topic"
)

type Sender interface {
	// Send takes a Request and returns the response or error
	Send(Request) (interface{}, error)
}

type ResponseHandler interface {
	// HandleResponse handles the response+error returned by the Sender
	HandleResponse(Request, interface{}, error) error
}

type ResponseHandleSetter interface {
	ResponseHandler() ResponseHandler
	SetResponseHandler(ResponseHandler)
}

type Connector interface {
	service.Startable
	service.Stopable
	service.Endpoint
	ResponseHandleSetter
	Manager() Manager
}

type connector struct {
	config  Config
	sender  Sender
	handler ResponseHandler
	manager Manager
	queue   Queue
	router  router.Router
	kvstore kvstore.KVStore

	ctx    context.Context
	cancel context.CancelFunc

	logger *log.Entry
	wg     sync.WaitGroup
}

type Config struct {
	Name       string
	Schema     string
	Prefix     string
	URLPattern string
	Workers    int
}

func NewConnector(router router.Router, sender Sender, config Config) (*connector, error) {
	kvs, err := router.KVStore()
	if err != nil {
		return nil, err
	}

	return &connector{
		config:  config,
		sender:  sender,
		manager: NewManager(config.Schema, kvs),
		queue:   NewQueue(sender, config.Workers),
		router:  router,
		kvstore: kvs,
		logger:  logger.WithField("name", config.Name),
	}, nil
}

// TODO Bogdan Refactor this so the router is built one time
func (c *connector) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r := mux.NewRouter()

	baseRouter := r.PathPrefix(c.GetPrefix()).Subrouter()
	baseRouter.Methods("GET").HandlerFunc(c.GetList)

	subRouter := baseRouter.Path(c.config.URLPattern).Subrouter()
	subRouter.Methods("POST").HandlerFunc(c.Post)
	subRouter.Methods("DELETE").HandlerFunc(c.Delete)

	r.ServeHTTP(w, req)
}

func (c *connector) GetPrefix() string {
	return c.config.Prefix
}

// GetList returns list of subscribers
func (c *connector) GetList(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	filters := make(map[string]string, len(query))

	for key, value := range query {
		if len(value) == 0 {
			continue
		}
		filters[key] = value[0]
	}

	subscribers := c.manager.Filter(filters)
	topics := make([]string, 0, len(subscribers))
	for _, s := range subscribers {
		topics = append(topics, string(s.Route().Path))
	}

	encoder := json.NewEncoder(w)
	err := encoder.Encode(topics)
	if err != nil {
		http.Error(w, "Error encoding data.", http.StatusInternalServerError)
		c.logger.WithField("error", err.Error()).Error("Error encoding data.")
		return
	}
}

// Post creates a new subscriber
func (c *connector) Post(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	topic, ok := params[TopicParam]
	if !ok {
		fmt.Fprintf(w, "Missing topic parameter.")
		return
	}
	delete(params, TopicParam)

	subscriber, err := c.manager.Create(protocol.Path(topic), params)
	if err != nil {
		if err == ErrSubscriberExists {
			fmt.Fprintf(w, `{"error":"subscription already exists"}`)
		} else {
			http.Error(w, fmt.Sprintf(`{"error":"unknown error: %s"}`, err.Error()), http.StatusInternalServerError)
		}
		return
	}

	go c.run(subscriber)
	fmt.Fprintf(w, `{"subscribed":"%v"}`, topic)
}

// Delete removes a subscriber
func (c *connector) Delete(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	topic, ok := params[TopicParam]
	if !ok {
		fmt.Fprintf(w, "Missing topic parameter.")
		return
	}

	delete(params, TopicParam)
	subscriber := c.manager.Find(GenerateKey(topic, params))
	if subscriber == nil {
		http.Error(w, `{"error":"subscription not found"}`, http.StatusNotFound)
		return
	}

	err := c.manager.Remove(subscriber)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"unknown error: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, `{"unsubscribed":"%v"}`, topic)
}

// Start will run start all current subscriptions and workers to process the messages
func (c *connector) Start() error {
	if c.queue == nil {
		return ErrInternalQueue
	}
	c.queue.Start()

	logger.Debug("Starting connector")
	c.ctx, c.cancel = context.WithCancel(context.Background())

	c.logger.Debug("Loading subscriptions")
	// Load subscriptions when starting
	err := c.manager.Load()
	if err != nil {
		return err
	}

	c.logger.Debug("Starting subscriptions")
	for _, s := range c.manager.List() {
		go c.run(s)
	}

	c.logger.Debug("Started connector")
	return nil
}

func (c *connector) run(s Subscriber) {
	c.wg.Add(1)
	defer c.wg.Done()

	var provideErr error
	go func() {
		err := s.Route().Provide(c.router, true)
		if err != nil {
			// cancel subscription loop if there is an error on the provider
			s.Cancel()
			provideErr = err
		}
	}()

	err := s.Loop(c.ctx, c.queue)
	if err != nil {
		c.logger.WithField("error", err.Error()).Error("Error returned by subscriber loop")
		// TODO Bogdan Handle different types of error eg. Closed route channel

		// TODO Bogdan Try restarting the subscription if possible
	}

	if provideErr != nil {
		// TODO Bogdan Treat errors where a subscription provide fails
		c.logger.WithField("error", err.Error()).Error("Route provide error")
	}
}

// Stop stops the connector (the context, the queue, the subscription loops)
func (c *connector) Stop() error {
	c.logger.Debug("Stopping connector")
	c.cancel()
	c.queue.Stop()
	c.wg.Wait()
	c.logger.Debug("Stopped connector")
	return nil
}

func (c *connector) Manager() Manager {
	return c.manager
}

func (c *connector) ResponseHandler() ResponseHandler {
	return c.handler
}

func (c *connector) SetResponseHandler(handler ResponseHandler) {
	c.handler = handler
	c.queue.SetResponseHandler(handler)
}
