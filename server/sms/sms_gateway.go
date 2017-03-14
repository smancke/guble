package sms

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/cosminrentea/gobbler/server/connector"

	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/cosminrentea/gobbler/protocol"
	"github.com/cosminrentea/gobbler/server/metrics"
	"github.com/cosminrentea/gobbler/server/router"
	"github.com/cosminrentea/gobbler/server/store"
)

const (
	SMSSchema       = "sms_notifications"
	SMSDefaultTopic = "/sms"
)

var (
	ErrRetryFailed = errors.New("Failed retrying to send message.")
)

type Sender interface {
	Send(*protocol.Message) error
}

type Config struct {
	Enabled         *bool
	APIKey          *string
	APISecret       *string
	Workers         *int
	SMSTopic        *string
	IntervalMetrics *bool

	Name   string
	Schema string
}

type gateway struct {
	config *Config

	sender Sender
	router router.Router
	route  *router.Route

	LastIDSent uint64

	ctx        context.Context
	cancelFunc context.CancelFunc

	logger *log.Entry
}

func New(router router.Router, sender Sender, config Config) (*gateway, error) {
	if *config.Workers <= 0 {
		*config.Workers = connector.DefaultWorkers
	}
	config.Schema = SMSSchema
	config.Name = SMSDefaultTopic
	return &gateway{
		config: &config,
		router: router,
		sender: sender,
		logger: logger.WithField("name", config.Name),
	}, nil
}

func (g *gateway) Start() error {
	g.logger.Debug("Starting gateway")

	err := g.ReadLastID()
	if err != nil {
		return err
	}

	g.ctx, g.cancelFunc = context.WithCancel(context.Background())
	g.initRoute()

	go g.Run()

	g.startMetrics()

	g.logger.Debug("Started gateway")
	return nil
}

func (g *gateway) initRoute() {
	g.route = router.NewRoute(router.RouteConfig{
		Path:         protocol.Path(*g.config.SMSTopic),
		ChannelSize:  5000,
		QueueSize:    -1,
		FetchRequest: g.fetchRequest(),
	})
}

func (g *gateway) fetchRequest() (fr *store.FetchRequest) {
	if g.LastIDSent > 0 {
		fr = store.NewFetchRequest(
			protocol.Path(*g.config.SMSTopic).Partition(),
			g.LastIDSent+1,
			0,
			store.DirectionForward, -1)
	}
	return
}

func (g *gateway) Run() {
	g.logger.Debug("Run gateway")
	var provideErr error
	go func() {
		err := g.route.Provide(g.router, true)
		if err != nil {
			// cancel subscription loop if there is an error on the provider
			logger.WithField("error", err.Error()).Error("Provide returned error")
			provideErr = err
			g.Cancel()
		}
	}()

	currentMsg, err := g.proxyLoop()
	if err != nil && provideErr == nil {
		g.logger.WithFields(log.Fields{
			"error":             err.Error(),
			"is_incomplete_sms": err == ErrIncompleteSMSSent,
		}).Error("Error returned by gateway proxy loop")

		if err == ErrIncompleteSMSSent {
			err2 := g.retry(currentMsg)
			if err2 != nil {
				g.logger.WithField("error", err2.Error()).Error("Error returned by retry.")
				if err3 := g.SetLastSentID(currentMsg.ID); err3 != nil {
					g.logger.WithField("error", err3.Error()).Error("Error setting last ID")
				}
			}
		}

		// If Route channel closed, try restarting
		if isRestartableErr(err) {
			g.Restart()
			return
		}

	}

	if provideErr != nil {
		// TODO Bogdan Treat errors where a subscription provide fails
		g.logger.WithField("error", provideErr.Error()).Error("Route provide error")

		// Router closed the route, try restart
		if provideErr == router.ErrInvalidRoute {
			g.Restart()
			return
		}
		// Router module is stopping, exit the process
		if _, ok := provideErr.(*router.ModuleStoppingError); ok {
			return
		}
	}
}
func isRestartableErr(err error) bool {
	return err == connector.ErrRouteChannelClosed ||
		err == ErrNoSMSSent ||
		err == ErrIncompleteSMSSent ||
		err == ErrSMSResponseDecodingFailed
}

// proxyLoop returns the current processed message alongside the error that
// occured during sending of the message
func (g *gateway) proxyLoop() (*protocol.Message, error) {
	var (
		opened      bool = true
		receivedMsg *protocol.Message
	)
	defer func() { g.cancelFunc = nil }()

	for opened {
		select {
		case receivedMsg, opened = <-g.route.MessagesChannel():
			if !opened {
				logger.WithField("receivedMsg", receivedMsg).Info("not open")
				break
			}

			err := g.send(receivedMsg)
			if err != nil {
				return receivedMsg, err
			}
		case <-g.ctx.Done():
			// If the parent context is still running then only this subscriber context
			// has been cancelled
			if g.ctx.Err() == nil {
				return nil, g.ctx.Err()
			}
			return nil, nil
		}
	}

	//TODO Cosmin Bogdan returning this error can mean 2 things: overflow of route's channel, or intentional stopping of router / gubled.
	return nil, connector.ErrRouteChannelClosed
}

func (g *gateway) retry(msg *protocol.Message) error {
	l := logger.WithField("message", msg)
	l.Info("Retrying to send message")
	for i := 0; i < 3; i++ {
		l.WithField("retry", i+1).Info("Sending message")
		err := g.send(msg)
		if err != nil {
			l.WithFields(log.Fields{
				"retry": i + 1,
				"err":   err.Error(),
			}).Error("Retry failed")
		} else {
			l.WithField("retry", i+1).Info("Retry success")
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return ErrRetryFailed
}

func (g *gateway) send(receivedMsg *protocol.Message) error {
	err := g.sender.Send(receivedMsg)
	if err != nil {
		log.WithField("error", err.Error()).Error("Sending of message failed")
		mTotalResponseErrors.Add(1)
		return err
	}
	mTotalSentMessages.Add(1)
	g.SetLastSentID(receivedMsg.ID)
	return nil
}

func (g *gateway) Restart() error {
	g.logger.WithField("LastIDSent", g.LastIDSent).Debug("Restart in progress")

	g.Cancel()
	g.cancelFunc = nil

	err := g.ReadLastID()
	if err != nil {
		return err
	}

	g.initRoute()

	go g.Run()

	g.logger.WithField("LastIDSent", g.LastIDSent).Debug("Restart finished")
	return nil
}

func (g *gateway) Stop() error {
	g.logger.Debug("Stopping gateway")
	g.cancelFunc()
	g.logger.Debug("Stopped gateway")
	return nil
}

func (g *gateway) SetLastSentID(ID uint64) error {
	g.logger.WithField("LastIDSent", ID).WithField("path", *g.config.SMSTopic).Debug("Seting LastIDSent")

	kvStore, err := g.router.KVStore()
	if err != nil {
		g.logger.WithField("error", err.Error()).Error("KVStore could not be accesed from gateway")
		return err
	}

	data, err := json.Marshal(struct{ ID uint64 }{ID: ID})
	if err != nil {
		g.logger.WithField("error", err.Error()).Error("Error encoding last ID")
		return err
	}
	err = kvStore.Put(g.config.Schema, *g.config.SMSTopic, data)
	if err != nil {
		g.logger.WithField("error", err.Error()).WithField("path", *g.config.SMSTopic).Error("KVStore could not set value for LastIDSent for topic")
		return err
	}
	g.LastIDSent = ID
	return nil
}

func (g *gateway) ReadLastID() error {
	kvStore, err := g.router.KVStore()
	if err != nil {
		g.logger.WithField("error", err.Error()).Error("KVStore could not be accesed from sms gateway")
		return err
	}
	data, exist, err := kvStore.Get(g.config.Schema, *g.config.SMSTopic)
	if err != nil {
		g.logger.WithField("error", err.Error()).WithField("path", *g.config.SMSTopic).Error("KvStore could not get value for LastIDSent for topic")
		return err
	}
	if !exist {
		g.LastIDSent = 0
		return nil
	}

	v := &struct{ ID uint64 }{}
	err = json.Unmarshal(data, v)
	if err != nil {
		g.logger.WithField("error", err.Error()).Error("Could not parse as uint64 the LastIDSent value stored in db")
		return err
	}
	g.LastIDSent = v.ID

	g.logger.WithField("LastIDSent", g.LastIDSent).WithField("path", *g.config.SMSTopic).Debug("ReadLastID")
	return nil
}

func (g *gateway) Cancel() {
	if g.cancelFunc != nil {
		g.cancelFunc()
	}
}

func (g *gateway) startMetrics() {
	mTotalSentMessages.Set(0)
	mTotalSendErrors.Set(0)
	mTotalResponseErrors.Set(0)
	mTotalResponseInternalErrors.Set(0)

	if *g.config.IntervalMetrics {
		g.startIntervalMetric(mMinute, time.Minute)
		g.startIntervalMetric(mHour, time.Hour)
		g.startIntervalMetric(mDay, time.Hour*24)
	}
}

func (g *gateway) startIntervalMetric(m metrics.Map, td time.Duration) {
	metrics.RegisterInterval(g.ctx, m, td, resetIntervalMetrics, processAndResetIntervalMetrics)
}
