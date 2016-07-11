package server

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/webserver"

	"net/http"
	"reflect"
	"time"

	"github.com/docker/distribution/health"
	"github.com/smancke/guble/metrics"
)

const (
	defaultHealthFrequency = time.Second * 60
	defaultHealthThreshold = 1
)

var loggerService = log.WithFields(log.Fields{
	"module": "service",
})

// Service is the main struct for controlling a guble server
type Service struct {
	webserver       *webserver.WebServer
	router          Router
	modules         []interface{}
	healthEndpoint  string
	healthFrequency time.Duration
	healthThreshold int
	metricsEndpoint string
}

// NewService creates a new Service, using the given Router and WebServer.
// If the router has already a configured Cluster, it is registered as a service module.
// The Router and Webserver are then registered as modules.
func NewService(router Router, webserver *webserver.WebServer) *Service {
	s := &Service{
		webserver:       webserver,
		router:          router,
		healthFrequency: defaultHealthFrequency,
		healthThreshold: defaultHealthThreshold,
	}
	cluster := router.Cluster()
	if cluster != nil {
		s.RegisterModules(cluster)
		router.Cluster().MessageHandler = router
	}
	s.RegisterModules(s.router, s.webserver)
	return s
}

// RegisterModules adds more modules (which can be Startable, Stopable, Endpoint etc.) to the service.
func (s *Service) RegisterModules(modules ...interface{}) {
	loggerService.WithFields(log.Fields{
		"numberOfNewModules":      len(modules),
		"numberOfExistingModules": len(s.modules),
	}).Info("RegisterModules: adding")

	s.modules = append(s.modules, modules...)
}

// HealthEndpoint sets the endpoint used for health. Parameter for disabling the endpoint is: "". Returns the updated service.
func (s *Service) HealthEndpoint(endpointPrefix string) *Service {
	s.healthEndpoint = endpointPrefix
	return s
}

// MetricsEndpoint sets the endpoint used for metrics. Parameter for disabling the endpoint is: "". Returns the updated service.
func (s *Service) MetricsEndpoint(endpointPrefix string) *Service {
	s.metricsEndpoint = endpointPrefix
	return s
}

// Start checks the modules for the following interfaces and registers and/or starts:
//   Startable:
//   health.Checker:
//   Endpoint: Register the handler function of the Endpoint in the http service at prefix
func (s *Service) Start() error {
	el := protocol.NewErrorList("service: errors occured while starting: ")

	if s.healthEndpoint != "" {
		loggerService.WithField("healthEndpoint", s.healthEndpoint).Info("Health endpoint")
		s.webserver.Handle(s.healthEndpoint, http.HandlerFunc(health.StatusHandler))
	} else {
		loggerService.Info("Health endpoint disabled")
	}

	if s.metricsEndpoint != "" {
		loggerService.WithField("metricsEndpoint", s.metricsEndpoint).Info("Metrics endpoint")
		s.webserver.Handle(s.metricsEndpoint, http.HandlerFunc(metrics.HttpHandler))
	} else {
		loggerService.Info("Metrics endpoint disabled")
	}

	for _, module := range s.modules {
		name := reflect.TypeOf(module).String()
		if startable, ok := module.(Startable); ok {
			loggerService.WithField("name", name).Info("Starting module")

			if err := startable.Start(); err != nil {
				loggerService.WithError(err).WithField("name", name).Error("Error while starting module")
				el.Add(err)
			}
		}
		if checker, ok := module.(health.Checker); ok && s.healthEndpoint != "" {
			loggerService.WithField("name", name).Info("Registering module as Health-Checker")
			health.RegisterPeriodicThresholdFunc(name, s.healthFrequency, s.healthThreshold, health.CheckFunc(checker.Check))
		}
		if endpoint, ok := module.(Endpoint); ok {
			prefix := endpoint.GetPrefix()
			loggerService.WithFields(log.Fields{
				"name":   name,
				"prefix": prefix,
			}).Info("Registering module as Endpoint")
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

	if s.router.Cluster() == nil {
		for i := 1; i < len(stopables); i++ {
			stopOrder[i] = len(stopables) - i
		}
	} else {
		stopOrder[0] = 1
		for i := 1; i < len(stopables)-1; i++ {
			stopOrder[i] = len(stopables) - i
		}
		stopOrder[len(stopables)-1] = 0
	}

	loggerService.WithField("stopOrder", stopOrder).Debug("Stopping modules in this order relative to registration")

	errors := protocol.NewErrorList("stopping errors: ")
	for _, order := range stopOrder {
		module := stopables[order]
		name := reflect.TypeOf(module).String()
		loggerService.WithFields(log.Fields{
			"name":  name,
			"order": order,
		}).Info("Stopping module")
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

// Router returns the service Router instance
func (s *Service) Router() Router {
	return s.router
}
