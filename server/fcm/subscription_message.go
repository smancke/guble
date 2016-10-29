package fcm

import (
	"encoding/json"
	"github.com/Bogh/gcm"
	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/protocol"
)

type subscriptionMessage struct {
	*subscription
	message *protocol.Message
	resultC chan *gcm.Response
	errC    chan error
}

func newSubscriptionMessage(s *subscription, m *protocol.Message) *subscriptionMessage {
	return &subscriptionMessage{
		s,
		m,
		make(chan *gcm.Response, 1),
		make(chan error, 1),
	}
}

func (sm *subscriptionMessage) fcmMessage() *gcm.Message {
	m := &gcm.Message{}
	err := json.Unmarshal(sm.message.Body, m)
	if err != nil {
		sm.subscription.logger.WithFields(log.Fields{
			"error":     err.Error(),
			"body":      string(sm.message.Body),
			"messageID": sm.message.ID,
		}).Debug("Could not decode gcm.Message from guble message body")
	} else if m.Notification != nil && m.Data != nil {
		return m
	}

	err = json.Unmarshal(sm.message.Body, &m.Data)
	if err != nil {
		m.Data = map[string]interface{}{
			"message": sm.message.Body,
		}
	}
	return m
}

func (sm *subscriptionMessage) closeChannels() {
	close(sm.resultC)
	close(sm.errC)
}

// ignoreMessage will send a errIgnoreMessage into the error channel so the processing can continue
func (sm *subscriptionMessage) ignoreMessage() {
	sm.errC <- errIgnoreMessage
}
