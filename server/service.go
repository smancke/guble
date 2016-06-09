package server

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/distribution/health"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/webserver"
	"net/http"
	"reflect"
	"time"
)

const (
	healthEndpointPrefix        = "/health"
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
	webserver            *webserver.WebServer
	router               Router
	modules              []interface{}
	healthCheckFrequency time.Duration
	healthCheckThreshold int
}

// NewService registers the Main Router, where other modules can subscribe for messages
func NewService(router Router, webserver *webserver.WebServer) *Service {
	service := &Service{
		webserver:            webserver,
		router:               router,
		healthCheckFrequency: defaultHealthCheckFrequency,
		healthCheckThreshold: defaultHealthCheckThreshold,
	}
	service.RegisterModules(service.router, service.webserver)
	return service
}

func (s *Service) RegisterModules(modules ...interface{}) {
	log.WithFields(log.Fields{
		"module":                 "service",
		"no_of_new_modules":      len(s.modules),
		"no of_existing_modules": len(modules),
	}).Debug(" RegisterModules: adding")

	s.modules = append(s.modules, modules...)
}

// Start checks the modules for the following interfaces and registers and/or starts:
//   Startable:
//   health.Checker:
//   Endpoint: Register the handler function of the Endpoint in the http service at prefix
func (s *Service) Start() error {
	el := protocol.NewErrorList("service: errors occured while starting: ")
	s.webserver.Handle(healthEndpointPrefix, http.HandlerFunc(health.StatusHandler))
	for _, module := range s.modules {
		name := reflect.TypeOf(module).String()
		if startable, ok := module.(Startable); ok {
			log.WithFields(log.Fields{
				"module": "service",
				"name":   name,
			}).Info("Starting module")

			if err := startable.Start(); err != nil {

				log.WithFields(log.Fields{
					"module": "service",
					"name":   name,
					"err":    err,
				}).Error("Error while starting module")

				el.Add(err)
			}
		}
		if checker, ok := module.(health.Checker); ok {

			log.WithFields(log.Fields{
				"module": "service",
				"name":   name,
			}).Info("Resgistering HealthChecker  module with name")
			health.RegisterPeriodicThresholdFunc(name, s.healthCheckFrequency, s.healthCheckThreshold, health.CheckFunc(checker.Check))
		}
		if endpoint, ok := module.(Endpoint); ok {
			prefix := endpoint.GetPrefix()
			log.WithFields(log.Fields{
				"module": "service",
				"name":   name,
				"prefix": prefix,
			}).Info("Resgistering Endpoint  module with name")
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
	log.WithFields(log.Fields{
		"module":        "service",
		"no_of_modules": len(stopOrder),
		"stop_order":    stopOrder,
	}).Debug(" Stopping modules in this order relative to registration")

	errors := protocol.NewErrorList("stopping errors: ")
	for _, order := range stopOrder {
		module := stopables[order]
		name := reflect.TypeOf(module).String()

		log.WithFields(log.Fields{
			"module": "service",
			"name":   name,
			"order":  order,
		}).Info("Stopping module with name")
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
