package gcm

import (
	"encoding/json"

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
	fcmJSON *fcmJSON
}

func newPipeMessage(s *subscription, m *protocol.Message) *pipeMessage {
	return &pipeMessage{s, m, make(chan *gcm.Response, 1), make(chan error, 1), nil}
}

func (pm *pipeMessage) data() map[string]interface{} {
	fcm, err := pm.fcm()
	if err == nil && fcm.Data != nil {
		return fcm.Data
	}

	jsonBody := make(map[string]interface{})
	err = json.Unmarshal(pm.message.Body, jsonBody)
	if err != nil {
		return map[string]interface{}{"message": pm.message.Body}
	}
	return jsonBody
}

func (pm *pipeMessage) notification() *gcm.Notification {
	fcm, err := pm.fcm()
	if err == nil && fcm.Notification != nil {
		return fcm.Notification
	}
	return nil
}

func (pm *pipeMessage) fcm() (*fcmJSON, error) {
	if pm.fcmJSON != nil {
		return pm.fcmJSON, nil
	}

	pm.fcmJSON = &fcmJSON{}
	err := json.Unmarshal(pm.message.Body, pm.fcmJSON)
	if err != nil {
		return nil, err
	}

	return pm.fcmJSON, nil
}

func (pm *pipeMessage) closeChannels() {
	close(pm.resultC)
	close(pm.errC)
}

// ignoreMessage will send a errIgnoreMessage in to the error channel so the processing can continue
func (pm *pipeMessage) ignoreMessage() {
	pm.errC <- errIgnoreMessage
}
