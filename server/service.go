package server

import (
	"github.com/smancke/guble/protocol"
	"github.com/docker/distribution/health"

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

// Module is an interface aggregating basic interfaces.
type Module interface {
	Startable
	Stopable
	health.Checker
}

// Endpoint adds a HTTP handler for the `GetPrefix()` to the webserver
type Endpoint interface {
	http.Handler
	GetPrefix() string
}

// Service is the main class for simple control of a server
type Service struct {
	webServer *WebServer
	router    Router
	modules   []Module
	// The time given to each Module on Stop()
	StopGracePeriod time.Duration
}

// NewService registers the Main Router, where other modules can subscribe for messages
func NewService(addr string, router Router) *Service {
	service := &Service{
		modules:         make([]Module, 0, 5),
		webServer:       NewWebServer(addr),
		router:          router,
		StopGracePeriod: time.Second * 2,
	}
	service.Register(service.webServer)
	service.Register(service.router)

	service.webServer.Handle("/health", http.HandlerFunc(health.StatusHandler))

	return service
}

// Register the supplied module on this service.
// This method checks the module for the following interfaces and does the expected registrations:
//   Module: if appropriate, add it to the slice of Modules
//   Endpoint: if appropriate, register the handler function of the Endpoint in the http service at prefix
func (service *Service) Register(i interface{}) {
	name := reflect.TypeOf(i).String()

	if m, ok := i.(Module); ok {
		protocol.Info("register module %v", name)
		service.modules = append(service.modules, m)
	}

	if e, ok := i.(Endpoint); ok {
		protocol.Info("register %v as Endpoint to %v", name, e.GetPrefix())
		service.webServer.Handle(e.GetPrefix(), e)
	}
}

func (service *Service) Start() error {
	el := protocol.NewErrorList("Errors occured while starting the service: ")
	for _, module := range service.modules {
		name := reflect.TypeOf(module).String()

		protocol.Debug("starting module %v", name)
		if err := module.Start(); err != nil {
			protocol.Err("error on starting module %v", name)
			el.Add(err)
		}

		protocol.Debug("registering health-check for module %v", name)
		//TODO parameterize / configure frequency and threshold
		health.RegisterPeriodicThresholdFunc(name, time.Second*60, 1, health.CheckFunc(module.Check))
	}
	return el.ErrorOrNil()
}

func (service *Service) Stop() error {
	errors := make(map[string]error)
	for _, stopable := range service.modules {
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

func (service *Service) WebServer() *WebServer {
	return service.webServer
}
