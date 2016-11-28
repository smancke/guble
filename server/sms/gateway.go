package gateway

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/connector"
	"github.com/smancke/guble/server/router"
	routerimport "github.com/smancke/guble/server/router"
)

type Sender interface {
	Send(*protocol.Message) error
}

type GatewayConfig struct {
	path    string
	Workers int
	Name    string
}

type gateway struct {
	config *GatewayConfig
	sender Sender
	router router.Router
	logger *log.Entry
	route  *router.Route

	ctx           context.Context
	cancel        context.CancelFunc
	LastIDFetched uint64
}

func NewGateway(router router.Router, sender Sender, config GatewayConfig) (gateway, error) {
	if config.Workers <= 0 {
		config.Workers = connector.DefaultWorkers
	}

	gw := &gateway{
		router: router,
		logger: logger.WithField("name", config.Name),
	}

	return gw
}

func (gw *gateway) Start() {
	r := routerimport.NewRoute(router.RouteConfig{
		Path:        protocol.Path(gw.config.path),
		ChannelSize: 10,
	})
	gw.route = r

	var provideErr error
	go func() {
		err := gw.route.Provide(gw.router, true)
		if err != nil {
			// cancel subscription loop if there is an error on the provider
			//provideErr = err
			logger.WithField("error", err.Error()).Error("Provide returned error")
			provideErr = err
			gw.Cancel()
		}
	}()

	err := gw.proxyLoop()
	if err != nil && provideErr == nil {
		gw.logger.WithField("error", err.Error()).Error("Error returned by gateway proxy loop")

		// If Route channel closed try restarting
		if err == connector.ErrRouteChannelClosed {
			gw.Restart()
			return
		}

	}

	if provideErr != nil {
		// TODO Bogdan Treat errors where a subscription provide fails
		gw.logger.WithField("error", provideErr.Error()).Error("Route provide error")

		// Router closed the route, try restart
		if provideErr == router.ErrInvalidRoute {
			gw.Restart()
			return
		}
		// Router module is stopping, exit the process
		if _, ok := provideErr.(*router.ModuleStoppingError); ok {
			return
		}
	}

}

func (gw *gateway) proxyLoop() error {

	var (
		opened bool = true
		m      *protocol.Message
	)
	sCtx, cancel := context.WithCancel(gw.ctx)
	gw.cancel = cancel
	defer func() { gw.cancel = nil }()

	for opened {
		select {
		case m, opened = <-gw.route.MessagesChannel():
			if !opened {
				break
			}

			err := gw.sender.Send(m)
			if err != nil {
				log.WithField("error", err.Error()).Error("Sending of message failed")
			}

		case <-sCtx.Done():
			// If the parent context is still running then only this subscriber context
			// has been cancelled
			if gw.ctx.Err() == nil {
				return sCtx.Err()
			}
			return nil
		}
	}

	//TODO Cosmin Bogdan returning this error can mean 2 things: overflow of route's channel, or intentional stopping of router / gubled.
	return connector.ErrRouteChannelClosed
}

func (gw *gateway) Restart() {

}

func (gw *gateway) Stop() {

}

func (gw *gateway) Cancel() {
	if gw.cancel != nil {
		gw.cancel()
	}
}
