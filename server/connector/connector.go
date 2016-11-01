package connector

import (
	"github.com/docker/distribution/health"
	"github.com/gorilla/mux"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/router"
	"net/http"
)

type Request interface {
	Subscriber() Subscriber
	Message() *protocol.Message
}

type Sender interface {
	// Send takes a Request and returns the response or an error
	Send(Request) (interface{}, error)
}

type ResponseHandler interface {
	// HandleResponse is used to pass the results from send: response + error
	HandleResponse(request Request, response interface{}, errSend error) error
}

type Connector interface {
	health.Checker
	SubscriptionManager
	ResponseHandler

	http.Handler

	Prefix() string
	Start() error
	Stop() error
}

type ConnectorConfig struct {
	Name   string
	Prefix string
	Url    string
}

type connector struct {
	SubscriptionManager

	config ConnectorConfig
	router router.Router
	sender Sender
}

func NewConnector(router router.Router, sender Sender, config ConnectorConfig) (*connector, error) {
	kvstore, err := router.KVStore()
	if err != nil {
		return nil, err
	}

	manager, err := NewSubscriptionManager(config.Name, kvstore)
	if err != nil {
		return nil, err
	}

	return &connector{
		SubscriptionManager: manager,
		config:              config,
		router:              router,
		sender:              sender,
	}, nil
}

func (c *connector) Prefix() string {
	return c.config.Prefix
}

// TODO Bogdan Refactor this so the router is built one time
func (c *connector) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r := mux.NewRouter()
	s := r.PathPrefix(c.Prefix()).Path(c.config.Url).Subrouter()

	s.Methods("GET").HandlerFunc(c.Get)
	s.Methods("POST").HandlerFunc(c.Post)
	s.Methods("PUT").HandlerFunc(c.Put)
	s.Methods("DELETE").HandlerFunc(c.Delete)

	r.ServeHTTP(w, req)
}

// Returns list of subscriptions
func (c *connector) Get(w http.ResponseWriter, req *http.Request) {

}

func (c *connector) Post(w http.ResponseWriter, req *http.Request) {

}

func (c *connector) Put(w http.ResponseWriter, req *http.Request) {

}

func (c *connector) Delete(w http.ResponseWriter, req *http.Request) {

}
