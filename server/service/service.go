package service

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/distribution/health"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/cosminrentea/gobbler/server/metrics"
	"github.com/cosminrentea/gobbler/server/router"
	"github.com/cosminrentea/gobbler/server/webserver"

	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHealthFrequency = time.Second * 60
	defaultHealthThreshold = 1
)

// Service is the main struct for controlling a guble server
type Service struct {
	webserver          *webserver.WebServer
	router             router.Router
	modules            []module
	healthEndpoint     string
	healthFrequency    time.Duration
	healthThreshold    int
	metricsEndpoint    string
	prometheusEndpoint string
	togglesEndpoint    string
}

// New creates a new Service, using the given Router and WebServer.
// If the router has already a configured Cluster, it is registered as a service module.
// The Router and Webserver are then registered as modules.
func New(router router.Router, webserver *webserver.WebServer) *Service {
	s := &Service{
		webserver:       webserver,
		router:          router,
		healthFrequency: defaultHealthFrequency,
		healthThreshold: defaultHealthThreshold,
	}
	cluster := router.Cluster()
	if cluster != nil {
		s.RegisterModules(1, 5, cluster)
		router.Cluster().Router = router
	}
	s.RegisterModules(2, 2, s.router)
	s.RegisterModules(3, 4, s.webserver)
	return s
}

// RegisterModules adds more modules (which can be Startable, Stopable, Endpoint etc.) to the service,
// with their start and stop ordering across all the service's modules.
func (s *Service) RegisterModules(startOrder int, stopOrder int, ifaces ...interface{}) {
	logger.WithFields(log.Fields{
		"numberOfNewModules":      len(ifaces),
		"numberOfExistingModules": len(s.modules),
	}).Info("RegisterModules")

	for _, i := range ifaces {
		m := module{
			iface:      i,
			startLevel: startOrder,
			stopLevel:  stopOrder,
		}
		s.modules = append(s.modules, m)
	}
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

// PrometheusEndpoint sets the endpoint used for Prometheus metrics. Parameter for disabling the endpoint is: "". Returns the updated service.
func (s *Service) PrometheusEndpoint(endpointPrefix string) *Service {
	s.prometheusEndpoint = endpointPrefix
	return s
}

// TogglesEndpoint sets the endpoint used for Feature-Toggles. Parameter for disabling the endpoint is: "". Returns the updated service.
func (s *Service) TogglesEndpoint(endpointPrefix string) *Service {
	s.togglesEndpoint = endpointPrefix
	return s
}

// Start the health-check, old-format metrics, and Prometheus metrics endpoint,
// and then check the modules for the following interfaces and registers and/or start:
//   Startable:
//   health.Checker:
//   Endpoint: Register the handler function of the Endpoint in the http service at prefix
func (s *Service) Start() error {
	var multierr *multierror.Error
	if s.healthEndpoint != "" {
		logger.WithField("healthEndpoint", s.healthEndpoint).Info("Health endpoint")
		s.webserver.Handle(s.healthEndpoint, http.HandlerFunc(health.StatusHandler))
	} else {
		logger.Info("Health endpoint disabled")
	}
	if s.metricsEndpoint != "" {
		logger.WithField("metricsEndpoint", s.metricsEndpoint).Info("Metrics endpoint")
		s.webserver.Handle(s.metricsEndpoint, http.HandlerFunc(metrics.HttpHandler))
	} else {
		logger.Info("Metrics endpoint disabled")
	}
	if s.prometheusEndpoint != "" {
		logger.WithField("prometheusEndpoint", s.prometheusEndpoint).Info("Prometheus metrics endpoint")
		s.webserver.Handle(s.prometheusEndpoint, promhttp.Handler())
	} else {
		logger.Info("Prometheus metrics endpoint disabled")
	}
	if s.togglesEndpoint != "" {
		logger.WithField("togglesEndpoint", s.togglesEndpoint).Info("Toggles endpoint")
		s.webserver.Handle(s.togglesEndpoint, http.HandlerFunc(s.togglesHandlerFunc))
	} else {
		logger.Info("Toggles endpoint disabled")
	}
	for order, iface := range s.ModulesSortedByStartOrder() {
		name := reflect.TypeOf(iface).String()
		if s, ok := iface.(Startable); ok {
			logger.WithFields(log.Fields{"name": name, "order": order}).Info("Starting module")
			if err := s.Start(); err != nil {
				logger.WithError(err).WithField("name", name).Error("Error while starting module")
				multierr = multierror.Append(multierr, err)
			}
		} else {
			logger.WithFields(log.Fields{"name": name, "order": order}).Debug("Module is not startable")
		}
		if c, ok := iface.(health.Checker); ok && s.healthEndpoint != "" {
			logger.WithField("name", name).Info("Registering module as Health-Checker")
			health.RegisterPeriodicThresholdFunc(name, s.healthFrequency, s.healthThreshold, health.CheckFunc(c.Check))
		}
		if e, ok := iface.(Endpoint); ok {
			prefix := e.GetPrefix()
			logger.WithFields(log.Fields{"name": name, "prefix": prefix}).Info("Registering module as Endpoint")
			s.webserver.Handle(prefix, e)
		}
	}
	return multierr.ErrorOrNil()
}

// Stop stops the registered modules in their given order
func (s *Service) Stop() error {
	var multierr *multierror.Error
	for order, iface := range s.modulesSortedBy(ascendingStopOrder) {
		name := reflect.TypeOf(iface).String()
		if s, ok := iface.(Stopable); ok {
			logger.WithFields(log.Fields{"name": name, "order": order}).Info("Stopping module")
			if err := s.Stop(); err != nil {
				multierr = multierror.Append(multierr, err)
			}
		} else {
			logger.WithFields(log.Fields{"name": name, "order": order}).Debug("Module is not stoppable")
		}
	}
	return multierr.ErrorOrNil()
}

// WebServer returns the service *webserver.WebServer instance
func (s *Service) WebServer() *webserver.WebServer {
	return s.webserver
}

// ModulesSortedByStartOrder returns the registered modules sorted by their startOrder property
func (s *Service) ModulesSortedByStartOrder() []interface{} {
	return s.modulesSortedBy(ascendingStartOrder)
}

// modulesSortedBy returns the registered modules sorted using a `by` criteria.
func (s *Service) modulesSortedBy(criteria by) []interface{} {
	var sorted []interface{}
	by(criteria).sort(s.modules)
	for _, m := range s.modules {
		sorted = append(sorted, m.iface)
	}
	return sorted
}

func (s *Service) togglesHandlerFunc(w http.ResponseWriter, r *http.Request) {
	logger.Info("togglesHandlerFunc")
	for key, values := range r.URL.Query() {
		if !toggleAllowed(key) {
			logger.WithField("key", key).Info("toggling this module is not explicitly allowed")
			continue
		}
		if len(values) != 1 {
			logger.WithField("key", key).Info("ignoring toggles parameter since it has more than one value")
			continue
		}
		value := values[0]
		enable, err := strconv.ParseBool(value)
		if err != nil {
			logger.WithFields(log.Fields{
				"key":   key,
				"value": value,
			}).Info("ignoring toggles single parameter since it is not boolean")
			continue
		}
		s.tryToggleModule(key, enable, w)
	}
}

func toggleAllowed(modulePackage string) bool {
	if modulePackage == "sms" {
		return true
	}
	return false
}

func (s *Service) tryToggleModule(searchedModulePackage string, enable bool, w http.ResponseWriter) {
	for _, iface := range s.ModulesSortedByStartOrder() {
		packagePath := reflect.TypeOf(iface).String()
		packagePathTokens := strings.Split(packagePath, ".")
		var modulePackage string
		if len(packagePathTokens) > 0 {
			modulePackage = strings.TrimPrefix(packagePathTokens[0], "*")
		}
		if searchedModulePackage != modulePackage {
			continue
		}
		s.toggleModule(modulePackage, enable, iface, w)
	}
}

func (s *Service) toggleModule(modulePackage string, enable bool, iface interface{}, w http.ResponseWriter) {
	le := logger.WithFields(log.Fields{
		"modulePackage": modulePackage,
		"enable":        enable,
	})
	if s, ok := iface.(Startable); ok && enable {
		le.Info("Starting module")
		if err := s.Start(); err != nil {
			le.WithError(err).Error("Error while starting module")
			w.Write([]byte(fmt.Sprintf("%s could not be started.\n", modulePackage)))
			return
		}
		w.Write([]byte(fmt.Sprintf("%s was successfully started.\n", modulePackage)))
	}
	if s, ok := iface.(Stopable); ok && !enable {
		le.Info("Stopping module")
		if err := s.Stop(); err != nil {
			le.WithError(err).Error("Error while stopping module")
			w.Write([]byte(fmt.Sprintf("%s could not be stopped.\n", modulePackage)))
			return
		}
		w.Write([]byte(fmt.Sprintf("%s was successfully stopped.\n", modulePackage)))
	}
}
