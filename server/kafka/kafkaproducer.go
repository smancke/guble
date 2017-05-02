package kafka

import (
	"github.com/Shopify/sarama"
	"github.com/cosminrentea/gobbler/server/configstring"
	"github.com/cosminrentea/gobbler/server/service"
	"time"
)

type Producer interface {
	service.Startable
	service.Stopable
	Report(topic string, bytes []byte, key string)
}

type Config struct {
	Brokers *configstring.List
}

type producer struct {
	Config

	asyncProducer sarama.AsyncProducer
}

func NewProducer(c Config) (Producer, error) {
	logger.WithField("config", c).Info("NewProducer")
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V0_10_1_0
	saramaConfig.Producer.Retry.Max = 10
	saramaConfig.Producer.Flush.Frequency = time.Second
	p, err := sarama.NewAsyncProducer(*c.Brokers, saramaConfig)
	if err != nil {
		logger.WithError(err).Error("Could not create AsyncProducer")
		return nil, err
	}
	return &producer{
		Config:        c,
		asyncProducer: p,
	}, nil
}

func (p *producer) Report(topic string, bytes []byte, key string) {
	logger.WithField("topic", topic).Debug("Reporting to Kafka topic")
	p.asyncProducer.Input() <- &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(bytes),
	}
}

func (p *producer) Start() error {
	logger.Info("Start")
	go func() {
		for err := range p.asyncProducer.Errors() {
			logger.WithError(err).Error("Could not write to Kafka")
		}
	}()
	return nil
}

func (p *producer) Stop() error {
	logger.Info("Stop")
	if err := p.asyncProducer.Close(); err != nil {
		logger.WithError(err).Error("Could not close Kafka Producer")
		return err
	}
	return nil
}
