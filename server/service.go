package server

import (
	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/store"

	"fmt"
	"net/http"
	"reflect"
	"time"
)

// This is the main class for simple startup of a server
type Service struct {
	kvStore         store.KVStore
	messageStore    store.MessageStore
	webServer       *WebServer
	messageSink     MessageSink
	router          PubSubSource
	stopListener    []Stopable
	startListener   []Startable
	accessManager   AccessManager
	// The time given to each Module on Stop()
	StopGracePeriod time.Duration
}

// Registers the Main Router, where other modules can subscribe for messages

func NewService(addr string, kvStore store.KVStore, messageStore store.MessageStore, messageSink MessageSink, router PubSubSource, accessManager AccessManager) *Service {
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

	switch m := module.(type) {
	case Stopable:
		guble.Info("register %v as StopListener", name)
		service.AddStopListener(m)
	}

	switch m := module.(type) {
	case Startable:
		guble.Info("register %v as StartListener", name)
		service.AddStartListener(m)
	}

	switch m := module.(type) {
	case Endpoint:
		guble.Info("register %v as Endpoint to %v", name, m.GetPrefix())
		service.AddHandler(m.GetPrefix(), m)
	}

	// do the injections ...

	switch m := module.(type) {
	case SetKVStore:
		guble.Debug("inject KVStore to %v", name)
		m.SetKVStore(service.kvStore)
	}

	switch m := module.(type) {
	case SetMessageStore:
		guble.Debug("inject MessageStore to %v", name)
		m.SetMessageStore(service.messageStore)
	}

	switch m := module.(type) {
	case SetRouter:
		guble.Debug("inject Router to %v", name)
		m.SetRouter(service.router)
	}

	switch m := module.(type) {
	case SetMessageEntry:
		guble.Debug("inject MessageEntry to %v", name)
		m.SetMessageEntry(service.messageSink)
	}

	switch m := module.(type) {
	case SetAccessManager:
		guble.Debug("inject AccessManager to %v", name)
		m.SetAccessManager(service.accessManager)
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
