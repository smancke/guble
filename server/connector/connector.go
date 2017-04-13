package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"

	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/router"
	"github.com/cosminrentea/gobbler/server/service"
)

const (
	DefaultWorkers = 1
	SubstitutePath = "/substitute/"
)

var (
	TopicParam     = "topic"
	ConnectorParam = "connector"
)

type Sender interface {
	// Send takes a Request and returns the response or error
	Send(Request) (interface{}, error)
}

type SenderSetter interface {
	Sender() Sender
	SetSender(Sender)
}

type Metadata struct {
	Latency time.Duration
}

type ResponseHandler interface {
	// HandleResponse handles the response+error (returned by a Sender)
	HandleResponse(Request, interface{}, *Metadata, error) error
}

type ResponseHandlerSetter interface {
	ResponseHandler() ResponseHandler
	SetResponseHandler(ResponseHandler)
}

type Runner interface {
	Run(Subscriber)
}

type Connector interface {
	service.Startable
	service.Stopable
	service.Endpoint
	SenderSetter
	ResponseHandlerSetter
	Runner
	Manager() Manager
	Context() context.Context
}

type ResponsiveConnector interface {
	Connector
	ResponseHandler
}

type connector struct {
	config  Config
	sender  Sender
	handler ResponseHandler
	manager Manager
	queue   Queue
	router  router.Router

	mux *mux.Router

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

func NewConnector(router router.Router, sender Sender, config Config) (Connector, error) {
	kvs, err := router.KVStore()
	if err != nil {
		return nil, err
	}

	if config.Workers <= 0 {
		config.Workers = DefaultWorkers
	}

	c := &connector{
		config:  config,
		sender:  sender,
		manager: NewManager(config.Schema, kvs),
		queue:   NewQueue(sender, config.Workers),
		router:  router,
		logger:  logger.WithField("name", config.Name),
	}
	c.initMuxRouter()
	return c, nil
}

func (c *connector) initMuxRouter() {
	muxRouter := mux.NewRouter()

	baseRouter := muxRouter.PathPrefix(c.GetPrefix()).Subrouter()
	baseRouter.Methods(http.MethodGet).HandlerFunc(c.GetList)
	baseRouter.Methods(http.MethodPost).PathPrefix(SubstitutePath).HandlerFunc(c.Substitute)

	subRouter := baseRouter.Path(c.config.URLPattern).Subrouter()
	subRouter.Methods(http.MethodPost).HandlerFunc(c.Post)
	subRouter.Methods(http.MethodDelete).HandlerFunc(c.Delete)
	c.mux = muxRouter
}

func (c *connector) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c.logger.WithFields(log.Fields{
		"path": req.URL.RequestURI(),
	}).Info("Handling HTTP request")
	c.mux.ServeHTTP(w, req)

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

	c.logger.WithField("filters", filters).Info("Get list of subscriptions")
	if len(filters) == 0 {
		http.Error(w, `{"error":"Missing filters"}`, http.StatusBadRequest)
		return
	}

	subscribers := c.manager.Filter(filters)
	topics := make([]string, 0, len(subscribers))
	for _, s := range subscribers {
		topics = append(topics, s.Route().Path.RemovePrefixSlash())
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
	c.logger.WithField("params", params).Info("POST subscription")
	topic, ok := params[TopicParam]
	if !ok {
		fmt.Fprintf(w, "Missing topic parameter.")
		return
	}
	delete(params, TopicParam)
	params[ConnectorParam] = c.config.Name
	c.logger.WithField("params", params).WithField("topic", topic).Info("Creating subscription")
	subscriber, err := c.manager.Create(protocol.Path("/"+topic), params)
	if err != nil {
		if err == ErrSubscriberExists {
			fmt.Fprintf(w, `{"error":"subscription already exists"}`)
		} else {
			http.Error(w, fmt.Sprintf(`{"error":"unknown error: %s"}`, err.Error()), http.StatusInternalServerError)
		}
		return
	}
	go c.Run(subscriber)
	c.logger.WithField("topic", topic).Info("Subscription created")
	fmt.Fprintf(w, `{"subscribed":"/%v"}`, topic)
}

// Delete removes a subscriber
func (c *connector) Delete(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	c.logger.WithField("params", params).Info("DELETE subscription")
	topic, ok := params[TopicParam]
	if !ok {
		fmt.Fprintf(w, "Missing topic parameter.")
		return
	}
	delete(params, TopicParam)
	params[ConnectorParam] = c.config.Name
	c.logger.WithField("params", params).WithField("topic", topic).Info("Finding subscription to delete it")
	subscriber := c.manager.Find(GenerateKey("/"+topic, params))
	if subscriber == nil {
		http.Error(w, `{"error":"subscription not found"}`, http.StatusNotFound)
		return
	}
	c.logger.WithField("params", params).WithField("topic", topic).Info("Deleting subscription")
	err := c.manager.Remove(subscriber)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"unknown error: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, `{"unsubscribed":"/%v"}`, topic)
}

func (c *connector) Substitute(w http.ResponseWriter, req *http.Request) {
	s := new(substitution)
	err := json.NewDecoder(req.Body).Decode(&s)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"json body could not be decoded: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	if !s.isValid() {
		http.Error(w, `{"error":"not all required values were supplied"}`, http.StatusBadRequest)
		return
	}

	filters := map[string]string{}
	filters[s.FieldName] = s.OldValue
	subscribers := c.manager.Filter(filters)
	totalSubscribersUpdated := 0
	for _, sub := range subscribers {
		sub.Route().Set(s.FieldName, s.NewValue)
		err = c.manager.Update(sub)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		totalSubscribersUpdated++
	}

	c.logger.WithField("subscribers", subscribers).WithField("req", s).Info("Substituted subscriber info ")
	fmt.Fprintf(w, `{"modified":"%d"}`, totalSubscribersUpdated)
}

// Start will run start all current subscriptions and workers to process the messages
func (c *connector) Start() error {
	c.queue.Start()

	c.logger.Info("Starting connector")
	c.ctx, c.cancel = context.WithCancel(context.Background())

	c.logger.Info("Loading subscriptions")
	err := c.manager.Load()
	if err != nil {
		return err
	}

	c.logger.Info("Starting subscriptions")
	for _, s := range c.manager.List() {
		go c.Run(s)
	}

	c.logger.Info("Started connector")
	return nil
}

func (c *connector) Run(s Subscriber) {
	c.wg.Add(1)
	defer c.wg.Done()

	var provideErr error
	go func() {
		err := s.Route().Provide(c.router, true)
		if err != nil {
			// cancel subscription loop if there is an error on the provider
			provideErr = err
			s.Cancel()
		}
	}()

	err := s.Loop(c.ctx, c.queue)
	if err != nil && provideErr == nil {
		c.logger.WithField("error", err.Error()).Error("Error returned by subscriber loop")
		// if context cancelled loop then unsubscribe the route from router
		// in case it's been subscribed
		if err == context.Canceled {
			c.router.Unsubscribe(s.Route())
			return
		}

		// If Route channel closed try restarting
		if err == ErrRouteChannelClosed {
			c.restart(s)
			return
		}
	}

	if provideErr != nil {
		// TODO Bogdan Treat errors where a subscription provide fails
		c.logger.WithField("error", provideErr.Error()).Error("Route provide error")

		// Router closed the route, try restart
		if provideErr == router.ErrInvalidRoute {
			c.restart(s)
			return
		}
		// Router module is stopping, exit the process
		if _, ok := provideErr.(*router.ModuleStoppingError); ok {
			return
		}
	}
}

func (c *connector) restart(s Subscriber) error {
	s.Cancel()
	err := s.Reset()
	if err != nil {
		c.logger.WithField("err", err.Error()).Error("Error reseting subscriber")
		return err
	}
	go c.Run(s)
	return nil
}

// Stop the connector (the context, the queue, the subscription loops)
func (c *connector) Stop() error {
	c.logger.Info("Stopping connector")
	c.cancel()
	c.queue.Stop()
	c.wg.Wait()
	c.logger.Info("Stopped connector")
	return nil
}

func (c *connector) Manager() Manager {
	return c.manager
}

func (c *connector) Context() context.Context {
	return c.ctx
}

func (c *connector) ResponseHandler() ResponseHandler {
	return c.handler
}

func (c *connector) SetResponseHandler(handler ResponseHandler) {
	c.handler = handler
	c.queue.SetResponseHandler(handler)
}

func (c *connector) Sender() Sender {
	return c.sender
}

func (c *connector) SetSender(s Sender) {
	c.sender = s
	c.queue.SetSender(s)
}
