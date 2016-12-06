package sms

import (
	"context"
	"encoding/binary"
	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/connector"
	"github.com/smancke/guble/server/router"
	routerimport "github.com/smancke/guble/server/router"
	"github.com/smancke/guble/server/store"
	"strconv"
)

const (
	SMSSchema = "sms_notifications"
	SMSDefaultTopic = "sms"
)

type Sender interface {
	Send(*protocol.Message) error
}

type Config struct {
	Enabled   *bool
	APIKey    *string
	APISecret *string
	Workers   *int
	SMSTopic  *string

	Name   string
	Schema string
}

type connector struct {
	config *Config
	sender Sender
	router router.Router
	logger *log.Entry
	route  *router.Route

	ctx        context.Context
	cancel     context.CancelFunc
	LastIDSent uint64
}

func New(router router.Router, sender Sender, config Config) (*connector, error) {
	if *config.Workers <= 0 {
		*config.Workers = connector.DefaultWorkers
	}
	config.Schema = SMSSchema
	config.Name = SMSDefaultTopic

	gw := &connector{
		router: router,
		logger: logger.WithField("name", config.Name),
		config: &config,
	}

	return gw, nil
}

func (gw *connector) Start() error {
	gw.logger.Debug("Starting gateway")

	err := gw.ReadLastID()
	if err != nil {
		return err
	}

	gw.ctx, gw.cancel = context.WithCancel(context.Background())

	var fr *store.FetchRequest
	if gw.LastIDSent == 0 {
		fr = store.NewFetchRequest(protocol.Path(*gw.config.SMSTopic).Partition(), 0, 0, store.DirectionForward, -1)
	} else {
		fr = store.NewFetchRequest(protocol.Path(*gw.config.SMSTopic).Partition(), gw.LastIDSent, 0, store.DirectionForward, -1)
	}

	r := routerimport.NewRoute(router.RouteConfig{
		Path:         protocol.Path(*gw.config.SMSTopic),
		ChannelSize:  10,
		FetchRequest: fr,
	})
	gw.route = r

	go gw.Run()

	gw.logger.Debug("Started gateway")
	return nil
}

func (gw *connector) Run() {
	gw.logger.Debug("Starting gateway run")
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

func (gw *connector) proxyLoop() error {
	var (
		opened bool = true
		m      *protocol.Message
	)
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
			gw.SetLastSentID(m.ID)

		case <-gw.ctx.Done():
			// If the parent context is still running then only this subscriber context
			// has been cancelled
			if gw.ctx.Err() == nil {
				return gw.ctx.Err()
			}
			return nil
		}
	}

	//TODO Cosmin Bogdan returning this error can mean 2 things: overflow of route's channel, or intentional stopping of router / gubled.
	return connector.ErrRouteChannelClosed
}

func (gw *connector) Restart() error {
	gw.Cancel()
	gw.cancel = nil

	err := gw.ReadLastID()
	if err != nil {
		return err
	}

	r := routerimport.NewRoute(router.RouteConfig{
		Path:         protocol.Path(*gw.config.SMSTopic),
		ChannelSize:  10,
		FetchRequest: store.NewFetchRequest(gw.route.Path.Partition(), gw.LastIDSent, 0, store.DirectionForward, -1),
	})
	gw.route = r

	go gw.Run()

	return nil
}

func (gw *connector) Stop() error {
	gw.logger.Debug("Stopping gateway")
	gw.cancel()
	gw.logger.Debug("Stopped gateway")
	return nil
}

func (gw *connector) SetLastSentID(ID uint64) error {
	gw.logger.WithField("lastID", ID).WithField("path", *gw.config.SMSTopic).Debug("Seting last id to ")

	kvStore, err := gw.router.KVStore()
	if err != nil {
		gw.logger.WithField("error", err.Error()).Error("KvStore could not be accesed from gateway")
		return err
	}

	buffer := make([]byte, 8)
	binary.LittleEndian.PutUint64(buffer, ID)
	err = kvStore.Put(gw.config.Schema, *gw.config.SMSTopic, buffer)
	if err != nil {
		gw.logger.WithField("error", err.Error()).WithField("path", *gw.config.SMSTopic).Error("KvStore could not set value for lastID for topic")
		return err
	}
	gw.LastIDSent = ID
	return nil
}

func (gw *connector) ReadLastID() error {
	kvStore, err := gw.router.KVStore()
	if err != nil {
		gw.logger.WithField("error", err.Error()).Error("KvStore could not be accesed from gateway")
		return err
	}
	val, exist, err := kvStore.Get(gw.config.Schema, *gw.config.SMSTopic)
	if err != nil {
		gw.logger.WithField("error", err.Error()).WithField("path", *gw.config.SMSTopic).Error("KvStore could not get value for lastID for topic")
		return err
	}

	if !exist {
		gw.LastIDSent = 0
		return nil
	}

	sequenceValue, err := strconv.ParseUint(string(val), 10, 64)
	if err != nil {
		gw.logger.WithField("error", err.Error()).Error("Could not parse to uint64 the value stored in db")
		return err
	}

	gw.LastIDSent = uint64(sequenceValue)

	gw.logger.WithField("lastID", gw.LastIDSent).WithField("path", *gw.config.SMSTopic).Debug("ReadLastID is ")
	return nil

}

func (gw *connector) Cancel() {
	if gw.cancel != nil {
		gw.cancel()
	}
}
