package server

import (
	"net/http"
)

// This is the main class for simple startup of a server
type Service struct {
	stopListener []Stopable
	webServer    *WSServer
}

func NewService(addr string) *Service {
	service := &Service{
		stopListener: make([]Stopable, 0, 5),
		webServer:    NewWebServer(addr),
	}
	service.AddStopListener(service.webServer)
	return service
}

func (service *Service) AddHandleFunc(prefix string, handler func(w http.ResponseWriter, r *http.Request)) {
	service.webServer.mux.HandleFunc(prefix, handler)
}

func (service *Service) Start() {
	service.webServer.Start()
}

func (service *Service) AddStopListener(stopable Stopable) {
	service.stopListener = append(service.stopListener, stopable)
}

func (service *Service) Stop() {
	for _, stopable := range service.stopListener {
		stopable.Stop()
	}
}

func (service *Service) GetWebServer() *WSServer {
	return service.webServer
}
