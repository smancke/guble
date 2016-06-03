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

const (
	healthEndpointPrefix        = "/health"
	defaultStopGracePeriod      = time.Second * 2
	defaultHealthCheckFrequency = time.Second * 60
	defaultHealthCheckThreshold = 1
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
	modules    []interface{}
	startables []Startable
	stopables  []Stopable
	// The time given to each Module on Stop()
	StopGracePeriod      time.Duration
	healthCheckFrequency time.Duration
	healthCheckThreshold int
}

// NewService registers the Main Router, where other modules can subscribe for messages
func NewService(router Router, webserver *webserver.WebServer) *Service {
	service := &Service{
		webserver:            webserver,
		router:               router,
		StopGracePeriod:      defaultStopGracePeriod,
		healthCheckFrequency: defaultHealthCheckFrequency,
		healthCheckThreshold: defaultHealthCheckThreshold,
	}
	service.Register(service.router)
	service.Register(service.webserver)

	service.webserver.Handle(healthEndpointPrefix, http.HandlerFunc(health.StatusHandler))

	return service
}

func (s *Service) RegisterModules(modules []interface{}) {
	for _, module := range modules {
		s.modules = append(s.modules, module)
		s.Register(module)
	}
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
		protocol.Info("service: registering %v as Startable", name)
		s.startables = append(s.startables, startable)
	}

	if stopable, ok := module.(Stopable); ok {
		protocol.Info("service: registering %v as Stopable", name)
		s.stopables = append(s.stopables, stopable)
	}

	if checker, ok := module.(health.Checker); ok {
		protocol.Info("service: registering %v as HealthChecker", name)
		health.RegisterPeriodicThresholdFunc(name, s.healthCheckFrequency, s.healthCheckThreshold, health.CheckFunc(checker.Check))
	}

	if endpoint, ok := module.(Endpoint); ok {
		prefix := endpoint.GetPrefix()
		protocol.Info("service: registering %v as Endpoint to %v", name, prefix)
		s.webserver.Handle(prefix, endpoint)
	}
}

func (s *Service) Start() error {
	el := protocol.NewErrorList("service: errors occured while starting: ")

	for _, startable := range s.startables {
		name := reflect.TypeOf(startable).String()

		protocol.Debug("service: starting module %v", name)
		if err := startable.Start(); err != nil {
			protocol.Err("service: error while starting module %v", name)
			el.Add(err)
		}
	}
	return el.ErrorOrNil()
}

func (s *Service) Stop() error {
	errors := make(map[string]error)
	for _, stopable := range s.stopables {
		name := reflect.TypeOf(stopable).String()
		stoppedC := make(chan bool)
		errorC := make(chan error)
		protocol.Info("service: stopping %v ...", name)
		go func() {
			err := stopable.Stop()
			if err != nil {
				errorC <- err
				return
			}
			stoppedC <- true
		}()
		select {
		case err := <-errorC:
			protocol.Err("service: error while stopping %v: %v", name, err.Error)
			errors[name] = err
		case <-stoppedC:
			protocol.Info("service: stopped %v", name)
		case <-time.After(s.StopGracePeriod):
			errors[name] = fmt.Errorf("service: error while stopping %v: did not stop after timeout %v", name, s.StopGracePeriod)
			protocol.Err(errors[name].Error())
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("service: errors while stopping modules: %q", errors)
	}
	return nil
}

func (s *Service) Modules() []interface{} {
	return s.modules
}

func (s *Service) WebServer() *webserver.WebServer {
	return s.webserver
}

// stop module with a timeout
func stopAsyncTimeout(m Stopable, timeout int) chan error {
	errC := make(chan err)
	go func() {
	}()
	return errC
}

// wait for channel to respond or until time expired
func wait() error {

}
