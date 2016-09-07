package gcm

import (
	"encoding/json"

	"github.com/Bogh/gcm"
	"github.com/smancke/guble/protocol"
)

func newPipeMessage(s *subscription, m *protocol.Message) *pipeMessage {
	return &pipeMessage{s, m, make(chan *gcm.Response, 1), make(chan error, 1), nil}
}

// Pipeline message
type pipeMessage struct {
	*subscription
	message *protocol.Message
	resultC chan *gcm.Response
	errC    chan error
	json    map[string]interface{}
}

func (pm *pipeMessage) data() map[string]interface{} {
	json := pm.jsonBody()
	if data, ok := json["data"]; ok {
		if mapData, ok := data.(map[string]interface{}); ok {
			return mapData
		}
		return map[string]interface{}{"message": data}
	}

	return map[string]interface{}{"message": pm.message.Body}
}

func (pm *pipeMessage) notification() *gcm.Notification {
	json := pm.jsonBody()

	if json == nil {
		return nil
	}

	if notification, ok := json["notification"]; ok {
		if gcmNotification, ok := notification.(gcm.Notification); ok {
			return &gcmNotification
		}
	}
	return nil
}

func (pm *pipeMessage) jsonBody() map[string]interface{} {
	if pm.json != nil {
		return pm.json
	}

	pm.json = make(map[string]interface{})
	err := json.Unmarshal(pm.message.Body, &pm.json)
	if err != nil {
		pm.logger.WithField("error", err.Error()).Debug("Cannot unmarshal message body to json object")
	}

	return pm.json
}

func (pm *pipeMessage) closeChannels() {
	close(pm.resultC)
	close(pm.errC)
}

// ignoreMessage will send a errIgnoreMessage in to the error channel so the processing can continue
func (pm *pipeMessage) ignoreMessage() {
	pm.errC <- errIgnoreMessage
}
