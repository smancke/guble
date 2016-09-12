package gcm

import (
	"encoding/json"

	log "github.com/Sirupsen/logrus"

	"github.com/Bogh/gcm"
	"github.com/smancke/guble/protocol"
)

type fcmJSON struct {
	Notification *gcm.Notification      `json:notification,omitempty`
	Data         map[string]interface{} `json:data`
}

// Pipeline message
type pipeMessage struct {
	*subscription
	message *protocol.Message
	resultC chan *gcm.Response
	errC    chan error
}

func newPipeMessage(s *subscription, m *protocol.Message) *pipeMessage {
	return &pipeMessage{s, m, make(chan *gcm.Response, 1), make(chan error, 1)}
}

func (pm *pipeMessage) fcmMessage() *gcm.Message {
	m := &gcm.Message{}
	err := json.Unmarshal(pm.message.Body, m)
	if err != nil {
		pm.subscription.logger.WithFields(log.Fields{
			"error":     err.Error(),
			"body":      string(pm.message.Body),
			"messageID": pm.message.ID,
		}).Debug("Could not decode gcm.Message from guble message body")
	} else if m.Notification != nil && m.Data != nil {
		return m
	}

	err = json.Unmarshal(pm.message.Body, &m.Data)
	if err != nil {
		m.Data = map[string]interface{}{
			"message": pm.message.Body,
		}
	}
	return m
}

func (pm *pipeMessage) closeChannels() {
	close(pm.resultC)
	close(pm.errC)
}

// ignoreMessage will send a errIgnoreMessage in to the error channel so the processing can continue
func (pm *pipeMessage) ignoreMessage() {
	pm.errC <- errIgnoreMessage
}
