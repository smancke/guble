package connector

import (
	"context"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/kvstore"
	"github.com/smancke/guble/server/router"
	"github.com/smancke/guble/server/service"
	"net/http"
	"sync"
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

type Connector interface {
	service.Startable
	service.Stopable
	service.Endpoint
}

type Conn struct {
	Config  Config
	Sender  Sender
	Handler ResponseHandler
	Manager Manager
	Queue   Queue
	Router  router.Router
	KVStore kvstore.KVStore

	Ctx    context.Context
	Cancel context.CancelFunc

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

func NewConnector(router router.Router, sender Sender, config Config) (*Conn, error) {
	kvs, err := router.KVStore()
	if err != nil {
		return nil, err
	}

	return &Conn{
		Config:  config,
		Sender:  sender,
		Manager: NewManager(config.Schema, kvs),
		Queue:   NewQueue(sender, config.Workers),
		Router:  router,
		KVStore: kvs,
		logger:  logger.WithField("name", config.Name),
	}, nil
}

// TODO Bogdan Refactor this so the router is built one time
func (c *Conn) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r := mux.NewRouter()

	baseRouter := r.PathPrefix(c.GetPrefix()).Subrouter()
	baseRouter.Methods("GET").HandlerFunc(c.GetList)

	subRouter := baseRouter.Path(c.Config.URLPattern).Subrouter()
	subRouter.Methods("POST").HandlerFunc(c.Post)
	subRouter.Methods("DELETE").HandlerFunc(c.Delete)

	r.ServeHTTP(w, req)
}

func (c *Conn) GetPrefix() string {
	return c.Config.Prefix
}

// GetList returns list of subscribers
func (c *Conn) GetList(w http.ResponseWriter, req *http.Request) {
	//TODO implement
}

// Post creates a new subscriber
func (c *Conn) Post(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	topic, ok := params[TopicParam]
	if !ok {
		fmt.Fprintf(w, "Missing topic parameter.")
		return
	}
	delete(params, TopicParam)

	subscriber, err := c.Manager.Create(protocol.Path(topic), params)
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
func (c *Conn) Delete(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	topic, ok := params[TopicParam]
	if !ok {
		fmt.Fprintf(w, "Missing topic parameter.")
		return
	}

	delete(params, TopicParam)
	subscriber := c.Manager.Find(GenerateKey(topic, params))
	if subscriber == nil {
		http.Error(w, `{"error":"subscription not found"}`, http.StatusNotFound)
		return
	}

	err := c.Manager.Remove(subscriber)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"unknown error: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, `{"unsubscribed":"%v"}`, topic)
}

// Start will run start all current subscriptions and workers to process the messages
func (c *Conn) Start() error {
	if c.Queue == nil {
		return ErrInternalQueue
	}
	c.Queue.Start()

	c.logger.Debug("Starting connector")
	c.Ctx, c.Cancel = context.WithCancel(context.Background())

	c.logger.Debug("Loading subscriptions")
	err := c.Manager.Load()
	if err != nil {
		return err
	}

	c.logger.Debug("Starting subscriptions")
	for _, s := range c.Manager.List() {
		go c.run(s)
	}

	c.logger.Debug("Started connector")
	return nil
}

func (c *Conn) run(s Subscriber) {
	c.wg.Add(1)
	defer c.wg.Done()

	var provideErr error
	go func() {
		err := s.Route().Provide(c.Router, true)
		if err != nil {
			// cancel subscription loop if there is an error on the provider
			s.Cancel()
			provideErr = err
		}
	}()

	err := s.Loop(c.Ctx, c.Queue)
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
func (c *Conn) Stop() error {
	c.logger.Debug("Stopping connector")
	c.Cancel()
	c.Queue.Stop()
	c.wg.Wait()
	c.logger.Debug("Stopped connector")
	return nil
}
