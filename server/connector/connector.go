package connector

import (
	"context"
	"errors"
	"github.com/gorilla/mux"
	"github.com/smancke/guble/server/kvstore"
	"github.com/smancke/guble/server/router"
	"github.com/smancke/guble/server/service"
	"net/http"
)

var (
	ErrInternalQueue = errors.New("internal queue should have been already created")
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
	Manager Manager
	Queue   Queue
	Router  router.Router
	KVStore kvstore.KVStore
	Ctx     context.Context
	Cancel  context.CancelFunc
}

type Config struct {
	Name   string
	Schema string
	Prefix string
	Url    string
}

func NewConnector(router router.Router, sender Sender, config Config) (Conn, error) {
	emptyConn := Conn{}
	kvs, err := router.KVStore()
	if err != nil {
		return emptyConn, err
	}

	manager, err := NewManager(config.Schema, kvs)
	if err != nil {
		return emptyConn, err
	}

	return Conn{
		Config:  config,
		Sender:  sender,
		Manager: manager,
		Router:  router,
		KVStore: kvs,
	}, nil
}

func (c *Conn) GetPrefix() string {
	return c.Config.Prefix
}

// TODO Bogdan Refactor this so the router is built one time
func (c *Conn) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r := mux.NewRouter()

	baseRouter := r.PathPrefix(c.GetPrefix()).Subrouter()
	baseRouter.Methods("GET").HandlerFunc(c.GetList)

	subRouter := baseRouter.Path(c.Config.Url).Subrouter()
	subRouter.Methods("POST").HandlerFunc(c.Post)
	subRouter.Methods("DELETE").HandlerFunc(c.Delete)

	r.ServeHTTP(w, req)
}

// GetList returns list of subscribers
func (c *Conn) GetList(w http.ResponseWriter, req *http.Request) {

}

// Post creates a new subscriber
func (c *Conn) Post(w http.ResponseWriter, req *http.Request) {
	// vars := mux.Vars(req)
}

// Delete removes a subscriber
func (c *Conn) Delete(w http.ResponseWriter, req *http.Request) {

}

// Start will run start all current subscriptions and workers to process the messages
func (c *Conn) Start() error {
	logger.WithField("name", c.Config.Name).Debug("Starting connector")
	c.Ctx, c.Cancel = context.WithCancel(context.Background())
	if c.Queue == nil {
		return ErrInternalQueue
	}
	c.Queue.Start()
	logger.WithField("name", c.Config.Name).Debug("Started connector")
	return nil
}

// Stop stops the context
func (c *Conn) Stop() error {
	logger.WithField("name", c.Config.Name).Debug("Stopping connector")
	// first cancel all subs-goroutines
	c.Cancel()
	// then close the queue:
	// - first the requests channel because push() will not be called anymore
	// - then the responses channel, after all the responses are received from the APNS service
	c.Queue.Stop()
	logger.WithField("name", c.Config.Name).Debug("Stopped connector")
	return nil
}
