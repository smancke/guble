package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"

	"fmt"
	"github.com/smancke/guble/server/auth"
	"net/http"
	"reflect"
	"time"
)

// This is the main class for simple startup of a server
type Service struct {
	kvStore       store.KVStore
	messageStore  store.MessageStore
	webServer     *WebServer
	messageSink   MessageSink
	router        PubSubSource
	stopListener  []Stopable
	startListener []Startable
	accessManager auth.AccessManager
	// The time given to each Module on Stop()
	StopGracePeriod time.Duration
}

// Registers the Main Router, where other modules can subscribe for messages

func NewService(addr string, kvStore store.KVStore, messageStore store.MessageStore, messageSink MessageSink, router PubSubSource, accessManager auth.AccessManager) *Service {
	service := &Service{
		stopListener:    make([]Stopable, 0, 5),
		kvStore:         kvStore,
		messageStore:    messageStore,
		webServer:       NewWebServer(addr),
		messageSink:     messageSink,
		router:          router,
		accessManager:   accessManager,
		StopGracePeriod: time.Second * 2,
	}
	service.Register(service.accessManager)
	service.Register(service.kvStore)
	service.Register(service.messageStore)
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

	if m, ok := module.(Startable); ok {
		guble.Info("register %v as StartListener", name)
		service.AddStartListener(m)
	}

	if m, ok := module.(Endpoint); ok {
		guble.Info("register %v as Endpoint to %v", name, m.GetPrefix())
		service.AddHandler(m.GetPrefix(), m)
	}

	if m, ok := module.(Stopable); ok {
		guble.Info("register %v as StopListener", name)
		service.AddStopListener(m)
	}

	// do the injections ...

	if m, ok := module.(SetMessageStore); ok {
		guble.Debug("inject MessageStore to %v", name)
		m.SetMessageStore(service.messageStore)
	}

	if m, ok := module.(SetRouter); ok {
		guble.Debug("inject Router to %v", name)
		m.SetRouter(service.router)
	}

	if m, ok := module.(SetMessageEntry); ok {
		guble.Debug("inject MessageEntry to %v", name)
		m.SetMessageEntry(service.messageSink)
	}
}

func (service *Service) AddHandler(prefix string, handler http.Handler) {
	service.webServer.mux.Handle(prefix, handler)
}

func (service *Service) Start() error {
	el := guble.NewErrorList("Errors occured while startup the service: ")

	for _, startable := range service.startListener {
		name := reflect.TypeOf(startable).String()

		guble.Debug("starting module %v", name)
		if err := startable.Start(); err != nil {
			guble.Err("error on startup module %v", name)
			el.Add(err)
		}
	}
	return el.ErrorOrNil()
}

func (service *Service) AddStopListener(stopable Stopable) {
	service.stopListener = append(service.stopListener, stopable)
}

func (service *Service) AddStartListener(startable Startable) {
	service.startListener = append(service.startListener, startable)
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
