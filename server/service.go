package server

import (
	"github.com/smancke/guble/guble"

	"net/http"
	"reflect"
)

// This is the main class for simple startup of a server
type Service struct {
	messageSink  MessageSink
	router       PubSubSource
	stopListener []Stopable
	webServer    *WebServer
}

// Registers the Main Router, where other modules can subscribe for messages

func NewService(addr string, router PubSubSource, messageSink MessageSink) *Service {
	service := &Service{
		stopListener: make([]Stopable, 0, 5),
		webServer:    NewWebServer(addr),
		messageSink:  messageSink,
		router:       router,
	}
	service.Register(service.webServer)
	service.Register(service.messageSink)
	service.Register(service.router)
	return service
}

// Registers the supplied module on this service.
// This method checks the module for the following interfaces and
// does the expected tegistrations:
//   Stopable: notify when the service stops
//   Endpoint: Register the handler function of the Endpoint in the http service at prefix
//   SetRouter: Provide the router
//   SetMessageEntry: Provide the message entry
//
// If the module does not have a HandlerFunc, the prefix parameter is ignored
func (service *Service) Register(module interface{}) {
	name := reflect.TypeOf(module).String()

	switch m := module.(type) {
	case Stopable:
		guble.Info("register %v as StopListener", name)
		service.AddStopListener(m)
	}

	switch m := module.(type) {
	case Endpoint:
		guble.Info("register %v as Endpoint to %v", name, m.GetPrefix())
		service.AddHandler(m.GetPrefix(), m)
	}

	switch m := module.(type) {
	case SetRouter:
		guble.Info("inject Router to %v", name)
		m.SetRouter(service.router)
	}

	switch m := module.(type) {
	case SetMessageEntry:
		guble.Info("inject MessageEntry to %v", name)
		m.SetMessageEntry(service.messageSink)
	}
}

func (service *Service) AddHandler(prefix string, handler http.Handler) {
	service.webServer.mux.Handle(prefix, handler)
}

func (service *Service) Start() {
	service.webServer.Start()
}

func (service *Service) AddStopListener(stopable Stopable) {
	service.stopListener = append(service.stopListener, stopable)
}

func (service *Service) Stop() {
	for _, stopable := range service.stopListener {
		stopable.Stop()
	}
}

func (service *Service) GetWebServer() *WebServer {
	return service.webServer
}
