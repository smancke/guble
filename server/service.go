package server

import (
	"github.com/smancke/guble/guble"

	"fmt"
	"net/http"
	"reflect"
	"time"
)

// This is the main class for simple startup of a server
type Service struct {
	messageSink  MessageSink
	router       PubSubSource
	stopListener []Stopable
	webServer    *WebServer
	// The time given to each Module on Stop()
	StopGracePeriod time.Duration
}

// Registers the Main Router, where other modules can subscribe for messages

func NewService(addr string, router PubSubSource, messageSink MessageSink) *Service {
	service := &Service{
		stopListener:    make([]Stopable, 0, 5),
		webServer:       NewWebServer(addr),
		messageSink:     messageSink,
		router:          router,
		StopGracePeriod: time.Second * 2,
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

func (service *Service) Stop() error {
	errors := make(map[string]error)
	for _, stopable := range service.stopListener {
		name := reflect.TypeOf(stopable).String()
		stoppedChan := make(chan bool)
		errorChan := make(chan error)
		guble.Info("stopping %v ...", name)
		go func() {
			err := stopable.Stop()
			if err != nil {
				errorChan <- err
				return
			}
			stoppedChan <- true
		}()
		select {
		case err := <-errorChan:
			guble.Err("error while stopping %v: %v", name, err.Error)
			errors[name] = err
		case <-stoppedChan:
			guble.Info("stopped %v", name)
		case <-time.After(service.StopGracePeriod):
			errors[name] = fmt.Errorf("error while stopping %v: not returned after %v seconds", name, service.StopGracePeriod)
			guble.Err(errors[name].Error())
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("Errors while stopping modules %q", errors)
	}
	return nil
}

func (service *Service) GetWebServer() *WebServer {
	return service.webServer
}
