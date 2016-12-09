package sms

import (
	"context"
	"encoding/binary"

	"github.com/smancke/guble/server/connector"

	"bytes"
	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/router"
	"github.com/smancke/guble/server/store"
)

const (
	SMSSchema       = "sms_notifications"
	SMSDefaultTopic = "/sms"
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

type conn struct {
	config *Config
	sender Sender
	router router.Router
	logger *log.Entry
	route  *router.Route

	ctx        context.Context
	cancel     context.CancelFunc
	LastIDSent uint64
}

func New(router router.Router, sender Sender, config Config) (*conn, error) {
	if *config.Workers <= 0 {
		*config.Workers = connector.DefaultWorkers
	}
	config.Schema = SMSSchema
	config.Name = SMSDefaultTopic

	gw := &conn{
		router: router,
		logger: logger.WithField("name", config.Name),
		config: &config,
		sender: sender,
	}

	return gw, nil
}

func (c *conn) Start() error {
	c.logger.Debug("Starting gateway")

	err := c.ReadLastID()
	if err != nil {
		return err
	}

	c.ctx, c.cancel = context.WithCancel(context.Background())

	var fr *store.FetchRequest
	if c.LastIDSent == 0 {
		fr = store.NewFetchRequest(protocol.Path(*c.config.SMSTopic).Partition(), 0, 0, store.DirectionForward, -1)
	} else {
		fr = store.NewFetchRequest(protocol.Path(*c.config.SMSTopic).Partition(), c.LastIDSent, 0, store.DirectionForward, -1)
	}

	r := router.NewRoute(router.RouteConfig{
		Path:         protocol.Path(*c.config.SMSTopic),
		ChannelSize:  10,
		FetchRequest: fr,
	})
	c.route = r

	go c.Run()

	c.logger.Debug("Started gateway")
	return nil
}

func (c *conn) Run() {
	c.logger.Debug("Starting gateway run")
	var provideErr error
	go func() {
		err := c.route.Provide(c.router, true)
		if err != nil {
			// cancel subscription loop if there is an error on the provider
			//provideErr = err
			logger.WithField("error", err.Error()).Error("Provide returned error")
			provideErr = err
			c.Cancel()
		}
	}()

	err := c.proxyLoop()
	if err != nil && provideErr == nil {
		c.logger.WithField("error", err.Error()).Error("Error returned by gateway proxy loop")

		// If Route channel closed try restarting
		if err == connector.ErrRouteChannelClosed || err == ErrNoSMSSent || err == ErrIncompleteSMSSent {
			c.Restart()
			return
		}

	}

	if provideErr != nil {
		// TODO Bogdan Treat errors where a subscription provide fails
		c.logger.WithField("error", provideErr.Error()).Error("Route provide error")

		// Router closed the route, try restart
		if provideErr == router.ErrInvalidRoute {
			log.Info("asgfagasg")
			c.Restart()
			return
		}
		// Router module is stopping, exit the process
		if _, ok := provideErr.(*router.ModuleStoppingError); ok {
			log.Info("asgfagasg 2")
			return
		}
	}

}

func (c *conn) proxyLoop() error {
	var (
		opened  bool = true
		recvMsg *protocol.Message
	)
	defer func() { c.cancel = nil }()

	for opened {
		select {
		case recvMsg, opened = <-c.route.MessagesChannel():
			if !opened {
				logger.WithField("m", recvMsg).Info("not open")
				break
			}

			err := c.sender.Send(recvMsg)
			if err != nil {
				log.WithField("error", err.Error()).Error("Sending of message failed")
				return err
			}
			c.SetLastSentID(recvMsg.ID)

		case <-c.ctx.Done():
			// If the parent context is still running then only this subscriber context
			// has been cancelled
			if c.ctx.Err() == nil {
				return c.ctx.Err()
			}
			return nil
		}
	}

	//TODO Cosmin Bogdan returning this error can mean 2 things: overflow of route's channel, or intentional stopping of router / gubled.
	return connector.ErrRouteChannelClosed
}

func (c *conn) Restart() error {
	c.logger.WithField("lastID", c.LastIDSent).Debug("Restart in progress")
	c.Cancel()
	c.cancel = nil

	err := c.ReadLastID()
	if err != nil {
		return err
	}

	r := router.NewRoute(router.RouteConfig{
		Path:         protocol.Path(*c.config.SMSTopic),
		ChannelSize:  10,
		FetchRequest: store.NewFetchRequest(c.route.Path.Partition(), c.LastIDSent, 0, store.DirectionForward, -1),
	})
	c.route = r

	go c.Run()
	c.logger.WithField("lastID", c.LastIDSent).Debug("Restart finished")
	return nil
}

func (c *conn) Stop() error {
	c.logger.Debug("Stopping gateway")
	c.cancel()
	c.logger.Debug("Stopped gateway")
	return nil
}

func (c *conn) SetLastSentID(ID uint64) error {
	c.logger.WithField("lastID", ID).WithField("path", *c.config.SMSTopic).Debug("Seting last id to ")

	kvStore, err := c.router.KVStore()
	if err != nil {
		c.logger.WithField("error", err.Error()).Error("KvStore could not be accesed from gateway")
		return err
	}

	buffer := make([]byte, 8)
	binary.LittleEndian.PutUint64(buffer, ID)
	err = kvStore.Put(c.config.Schema, *c.config.SMSTopic, buffer)
	if err != nil {
		c.logger.WithField("error", err.Error()).WithField("path", *c.config.SMSTopic).Error("KvStore could not set value for lastID for topic")
		return err
	}
	c.LastIDSent = ID
	return nil
}

func (c *conn) ReadLastID() error {
	kvStore, err := c.router.KVStore()
	if err != nil {
		c.logger.WithField("error", err.Error()).Error("KvStore could not be accesed from gateway")
		return err
	}
	val, exist, err := kvStore.Get(c.config.Schema, *c.config.SMSTopic)
	if err != nil {
		c.logger.WithField("error", err.Error()).WithField("path", *c.config.SMSTopic).Error("KvStore could not get value for lastID for topic")
		return err
	}

	if !exist {
		c.LastIDSent = 0
		return nil
	}

	sequenceValue, err := binary.ReadUvarint(bytes.NewBuffer(val))
	if err != nil {
		c.logger.WithField("error", err.Error()).Error("Could not parse to uint64 the value stored in db")
		return err
	}

	c.LastIDSent = uint64(sequenceValue)

	c.logger.WithField("lastID", c.LastIDSent).WithField("path", *c.config.SMSTopic).Debug("ReadLastID is ")
	return nil

}

func (c *conn) Cancel() {
	if c.cancel != nil {
		c.cancel()
	}
}
