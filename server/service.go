package server

import (
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/webserver"

	"fmt"
	"github.com/docker/distribution/health"
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
	webserver  *webserver.WebServer
	router     Router
	stopables  []Stopable
	startables []Startable
	// The time given to each Module on Stop()
	StopGracePeriod      time.Duration
	healthCheckFrequency time.Duration
	healthCheckThreshold int
}

// NewService registers the Main Router, where other modules can subscribe for messages
func NewService(router Router, webserver *webserver.WebServer) *Service {
	service := &Service{
		stopables:            make([]Stopable, 0, 5),
		webserver:            webserver,
		router:               router,
		StopGracePeriod:      time.Second * 2,
		healthCheckFrequency: time.Second * 60,
		healthCheckThreshold: 1,
	}
	service.Register(service.router)
	service.Register(service.webserver)

	service.webserver.Handle("/health", http.HandlerFunc(health.StatusHandler))

	return service
}

// Register the supplied module on this service.
// This method checks the module for the following interfaces and
// does the expected registrations:
//   Startable,
//   Stopable: notify when the service stops
//   health.Checker:
//   Endpoint: Register the handler function of the Endpoint in the http service at prefix
func (s *Service) Register(module interface{}) {
	name := reflect.TypeOf(module).String()

	if startable, ok := module.(Startable); ok {
		protocol.Info("register %v as Startable", name)
		s.startables = append(s.startables, startable)
	}

	if stopable, ok := module.(Stopable); ok {
		protocol.Info("register %v as Stopable", name)
		s.stopables = append(s.stopables, stopable)
	}

	if checker, ok := module.(health.Checker); ok {
		protocol.Info("register %v as HealthChecker", name)
		health.RegisterPeriodicThresholdFunc(name, s.healthCheckFrequency, s.healthCheckThreshold, health.CheckFunc(checker.Check))
	}

	if endpoint, ok := module.(Endpoint); ok {
		prefix := endpoint.GetPrefix()
		protocol.Info("register %v as Endpoint to %v", name, prefix)
		s.webserver.Handle(prefix, endpoint)
	}
}

func (s *Service) Start() error {
	el := protocol.NewErrorList("Errors occured while startup the service: ")

	for _, startable := range s.startables {
		name := reflect.TypeOf(startable).String()

		protocol.Debug("starting module %v", name)
		if err := startable.Start(); err != nil {
			protocol.Err("error on startup module %v", name)
			el.Add(err)
		}
	}
	return el.ErrorOrNil()
}

func (s *Service) Stop() error {
	errors := make(map[string]error)
	for _, stopable := range s.stopables {
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
		case <-time.After(s.StopGracePeriod):
			errors[name] = fmt.Errorf("error while stopping %v: not returned after %v seconds", name, s.StopGracePeriod)
			protocol.Err(errors[name].Error())
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("Errors while stopping modules %q", errors)
	}
	return nil
}

func (s *Service) WebServer() *webserver.WebServer {
	return s.webserver
}
