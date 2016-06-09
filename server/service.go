package server

import (
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/webserver"

	"github.com/docker/distribution/health"

	"fmt"
	"net/http"
	"reflect"
	"time"
)

const (
	defaultHealthFrequency = time.Second * 60
	defaultHealthThreshold = 1
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
	webserver             *webserver.WebServer
	router                Router
	modules               []interface{}
	healthEndpointPrefix  string
	healthFrequency       time.Duration
	healthThreshold       int
	metricsEndpointPrefix string
}

// NewService registers the Main Router, where other modules can subscribe for messages
func NewService(router Router, webserver *webserver.WebServer) *Service {
	service := &Service{
		webserver:       webserver,
		router:          router,
		healthFrequency: defaultHealthFrequency,
		healthThreshold: defaultHealthThreshold,
	}
	service.RegisterModules(service.router, service.webserver)
	return service
}

func (s *Service) RegisterModules(modules ...interface{}) {
	protocol.Debug("service: RegisterModules: adding %d modules after existing %d modules", len(modules), len(s.modules))
	s.modules = append(s.modules, modules...)
}

func (s *Service) HealthEndpointPrefix(value string) *Service {
	s.healthEndpointPrefix = value
	return s
}

func (s *Service) HealthFrequency(value time.Duration) *Service {
	s.healthFrequency = value
	return s
}

func (s *Service) HealthThreshold(value int) *Service {
	s.healthThreshold = value
	return s
}

func (s *Service) MetricsEndpointPrefix(value string) *Service {
	s.metricsEndpointPrefix = value
	return s
}

// Start checks the modules for the following interfaces and registers and/or starts:
//   Startable:
//   health.Checker:
//   Endpoint: Register the handler function of the Endpoint in the http service at prefix
func (s *Service) Start() error {
	el := protocol.NewErrorList("service: errors occured while starting: ")

	if s.healthEndpointPrefix != "" {
		protocol.Info("service: health endpoint: %v", s.healthEndpointPrefix)
		s.webserver.Handle(s.healthEndpointPrefix, http.HandlerFunc(health.StatusHandler))
	}

	if s.metricsEndpointPrefix != "" {
		protocol.Info("service: metrics endpoint: %v", s.metricsEndpointPrefix)
		s.webserver.Handle(s.metricsEndpointPrefix, http.HandlerFunc(expvarHandler))
	}

	for _, module := range s.modules {
		name := reflect.TypeOf(module).String()
		if startable, ok := module.(Startable); ok {
			protocol.Info("service: starting module %v", name)
			if err := startable.Start(); err != nil {
				protocol.Err("service: error while starting module %v", name)
				el.Add(err)
			}
		}
		if checker, ok := module.(health.Checker); ok && s.healthEndpointPrefix != "" {
			protocol.Info("service: registering module %v as HealthChecker", name)
			health.RegisterPeriodicThresholdFunc(name, s.healthFrequency, s.healthThreshold, health.CheckFunc(checker.Check))
		}
		if endpoint, ok := module.(Endpoint); ok {
			prefix := endpoint.GetPrefix()
			protocol.Info("service: registering module %v as Endpoint to %v", name, prefix)
			s.webserver.Handle(prefix, endpoint)
		}
	}
	return el.ErrorOrNil()
}

// Stop stops the registered modules in the required order
func (s *Service) Stop() error {
	var stopables []Stopable

	for _, module := range s.modules {
		if stopable, ok := module.(Stopable); ok {
			stopables = append(stopables, stopable)
		}
	}

	stopOrder := make([]int, len(stopables))
	for i := 1; i < len(stopables); i++ {
		stopOrder[i] = len(stopables) - i
	}

	protocol.Debug("service: stopping %d modules in this order relative to registration: %v", len(stopOrder), stopOrder)

	errors := protocol.NewErrorList("stopping errors: ")
	for _, order := range stopOrder {
		module := stopables[order]
		name := reflect.TypeOf(module).String()

		protocol.Info("service: stopping [%d] %v", order, name)
		if err := module.Stop(); err != nil {
			errors.Add(err)
		}
	}

	if err := errors.ErrorOrNil(); err != nil {
		return fmt.Errorf("service: errors while stopping modules: %s", err)
	}

	return nil
}

// Modules returns the slice of registered modules
func (s *Service) Modules() []interface{} {
	return s.modules
}

// WebServer returns the service *webserver.WebServer instance
func (s *Service) WebServer() *webserver.WebServer {
	return s.webserver
}
