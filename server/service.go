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
	defaultStopGracePeriod      = time.Second * 5
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
	StopGracePeriod      time.Duration // The timeout given to each Module on Stop()
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
	service.registerModule(service.router)
	service.registerModule(service.webserver)

	return service
}

func (s *Service) RegisterModules(modules []interface{}) {
	for _, module := range modules {
		s.registerModule(module)
	}
}

func (s *Service) registerModule(module interface{}) {
	s.modules = append(s.modules, module)
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
			protocol.Info("service: starting module %v", name)
			if err := startable.Start(); err != nil {
				protocol.Err("service: error while starting module %v", name)
				el.Add(err)
			}
		}
		if checker, ok := module.(health.Checker); ok {
			protocol.Info("service: registering module %v as HealthChecker", name)
			health.RegisterPeriodicThresholdFunc(name, s.healthCheckFrequency, s.healthCheckThreshold, health.CheckFunc(checker.Check))
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
		name := reflect.TypeOf(module).String()
		if stopable, ok := module.(Stopable); ok {
			protocol.Info("service: %v is Stopable", name)
			stopables = append(stopables, stopable)
		}
	}
	// stopOrder allows the customized stopping of the modules
	// (not necessarily in the exact reverse order of their registrations).
	// Router is first to stop, then the rest of the modules are stopped in reverse-registration-order.
	stopOrder := make([]int, len(stopables))
	for i := 1; i < len(stopables); i++ {
		stopOrder[i] = len(stopables) - i
	}

	protocol.Debug("service: stopping %d modules with a %v timeout, in this order relative to registration: %v",
		len(stopOrder), s.StopGracePeriod, stopOrder)
	errors := make(map[string]error)
	for _, order := range stopOrder {
		err := stopModule(stopables[order])
		if err != nil {
			errors[name] = err
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("service: errors while stopping modules: %q", errors)
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

func stopModule(stopable Stopable) error {
	name := reflect.TypeOf(stopable).String()
	protocol.Info("service: stopping [%d] %v", order, name)

	if _, ok := stopable.(Router); ok {
		protocol.Debug("service: %v is a Router and requires a blocking stop", name)
		return stopable.Stop()
	}
	return stopModuleWithTimeout(stopable, name, s.StopGracePeriod)
}

// stopWithTimeout waits for channel to respond with an error, or until time expires - and returns an error.
// If Stopable stopped correctly, it returns nil.
func stopModuleWithTimeout(stopable Stopable, name string, timeout time.Duration) error {
	select {
	case err := <-stopChannel(stopable):
		if err != nil {
			protocol.Err("service: error while stopping %v: %v", name, err.Error)
			return err
		}
	case <-time.After(timeout):
		errTimeout := fmt.Errorf("service: error while stopping %v: did not stop after timeout %v", name, timeout)
		protocol.Err(errTimeout.Error())
		return errTimeout
	}
	protocol.Info("service: stopped %v", name)
	return nil
}

func stopChannel(stopable Stopable) chan error {
	errorC := make(chan error)
	go func() {
		err := stopable.Stop()
		if err != nil {
			errorC <- err
		}
		close(errorC)
	}()
	return errorC
}
