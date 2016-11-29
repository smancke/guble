package gateway

import (
	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/protocol"
)

type OptivoSender struct {
	logger *log.Entry
}

func NewOptivoSender() Sender {
	return &OptivoSender{
		logger: logger.WithField("name", "optivoSender"),
	}
}

func (os *OptivoSender) Send(msg *protocol.Message) error {
	os.logger.WithFields(
		"ID", msg.ID,
		"body", msg.Body,
	).Info("Sending Message  to Optivo")
	return nil
}
