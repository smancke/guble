package server

import (
	"github.com/smancke/guble/protocol"

	"fmt"
	"net/http"
	"reflect"
	"time"
)

// Startable interface for modules which provide a start mechanism
type Startable interface {
	Start() error
}

// Stopable interface for modules which provide a stop mechanism
type Stopable interface {
	Stop() error
}

// Endpoint adds a HTTP handler for the `GetPrefix()` to the webserver
type Endpoint interface {
	http.Handler
	GetPrefix() string
}

// Service is the main class for simple control of a server
type Service struct {
	webServer  *WebServer
	router     Router
	stopables  []Stopable
	startables []Startable
	// The time given to each Module on Stop()
	StopGracePeriod time.Duration
}

// NewService registers the Main Router, where other modules can subscribe for messages
func NewService(
	addr string,
	router Router) *Service {
	service := &Service{
		stopables:       make([]Stopable, 0, 5),
		webServer:       NewWebServer(addr),
		router:          router,
		StopGracePeriod: time.Second * 2,
	}
	service.Register(service.webServer)
	service.Register(service.router)

	return service
}

// Register the supplied module on this service.
// This method checks the module for the following interfaces and
// does the expected registrations:
//   Startable: notify when the service stops
//   Stopable: notify when the service stops
//   Endpoint: Register the handler function of the Endpoint in the http service at prefix
func (service *Service) Register(module interface{}) {
	name := reflect.TypeOf(module).String()

	if m, ok := module.(Startable); ok {
		protocol.Info("register %v as StartListener", name)
		service.AddStartListener(m)
	}

	if m, ok := module.(Endpoint); ok {
		protocol.Info("register %v as Endpoint to %v", name, m.GetPrefix())
		service.AddHandler(m.GetPrefix(), m)
	}

	if m, ok := module.(Stopable); ok {
		protocol.Info("register %v as StopListener", name)
		service.AddStopListener(m)
	}
}

func (service *Service) Start() error {
	el := protocol.NewErrorList("Errors occured while startup the service: ")

	for _, startable := range service.startables {
		name := reflect.TypeOf(startable).String()

		protocol.Debug("starting module %v", name)
		if err := startable.Start(); err != nil {
			protocol.Err("error on startup module %v", name)
			el.Add(err)
		}
	}
	return el.ErrorOrNil()
}

func (service *Service) Stop() error {
	errors := make(map[string]error)
	for _, stopable := range service.stopables {
		name := reflect.TypeOf(stopable).String()
		stoppedChan := make(chan bool)
		errorChan := make(chan error)
		protocol.Info("stopping %v ...", name)
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
			protocol.Err("error while stopping %v: %v", name, err.Error)
			errors[name] = err
		case <-stoppedChan:
			protocol.Info("stopped %v", name)
		case <-time.After(service.StopGracePeriod):
			errors[name] = fmt.Errorf("error while stopping %v: not returned after %v seconds", name, service.StopGracePeriod)
			protocol.Err(errors[name].Error())
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("Errors while stopping modules %q", errors)
	}
	return nil
}

func (service *Service) AddStopListener(stopable Stopable) {
	service.stopables = append(service.stopables, stopable)
}

func (service *Service) AddStartListener(startable Startable) {
	service.startables = append(service.startables, startable)
}

func (service *Service) AddHandler(prefix string, handler http.Handler) {
	service.webServer.mux.Handle(prefix, handler)
}

func (service *Service) GetWebServer() *WebServer {
	return service.webServer
}
