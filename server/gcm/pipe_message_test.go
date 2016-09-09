package gcm

import (
	"testing"

	log "github.com/Sirupsen/logrus"

	"github.com/smancke/guble/protocol"
	"github.com/stretchr/testify/assert"
)

var fullFCMMessage = `{
	"notification": {
		"title": "TEST",
		"body": "notification body",
		"icon": "ic_notification_test_icon",
		"click_action": "estimated_arrival"
	},
	"data": {"field1": "value1", "field2": "value2"}
}`

func TestPipeMessage_fcmMessage(t *testing.T) {
	a := assert.New(t)

	m := &protocol.Message{
		Body: []byte(fullFCMMessage),
	}
	s := &subscription{logger: log.WithField("test", "on")}
	pm := newPipeMessage(nil, m)
	fcmMessage := pm.fcmMessage()

	a.NotNil(fcmMessage.Notification)
	a.Equal("TEST", fcmMessage.Notification.Title)
	a.Equal("notification body", fcmMessage.Notification.Body)
	a.Equal("ic_notification_test_icon", fcmMessage.Notification.Icon)
	a.Equal("estimated_arrival", fcmMessage.Notification.ClickAction)
	a.NotNil(fcmMessage.Data)
	a.IsType(map[string]interface{}{}, fcmMessage.Data)
	if a.Contains(fcmMessage.Data, "field1") {
		a.Equal("value1", fcmMessage.Data["field1"])
	}
	if a.Contains(fcmMessage.Data, "field2") {
		a.Equal("value2", fcmMessage.Data["field2"])
	}

	m = &protocol.Message{
		Body: []byte(`{"field1": "value1", "field2": "value2"}`),
	}
	pm = newPipeMessage(s, m)
	fcmMessage = pm.fcmMessage()
	a.Nil(fcmMessage.Notification)
	a.IsType(map[string]interface{}{}, fcmMessage.Data)
	if a.Contains(fcmMessage.Data, "field1") {
		a.Equal("value1", fcmMessage.Data["field1"])
	}
	if a.Contains(fcmMessage.Data, "field2") {
		a.Equal("value2", fcmMessage.Data["field2"])
	}

	m = &protocol.Message{
		Body: []byte(`plain body message`),
	}
	pm = newPipeMessage(s, m)
	fcmMessage = pm.fcmMessage()
	a.Nil(fcmMessage.Notification)
	a.IsType(map[string]interface{}{}, fcmMessage.Data)
	if a.Contains(fcmMessage.Data, "message") {
		a.Equal([]byte("plain body message"), fcmMessage.Data["message"])
	}
}

// func TestPipeMessage_data(t *testing.T) {
// 	a := assert.New(t)

// 	m := &protocol.Message{
// 		Body: []byte(fullFCMMessage),
// 	}
// 	pm := newPipeMessage(nil, m)
// 	data := pm.data()
// 	a.NotNil(data)
// 	a.IsType(map[string]interface{}{}, data)
// 	if a.Contains(data, "field1") {
// 		a.Equal("value1", data["field1"])
// 	}
// 	if a.Contains(data, "field2") {
// 		a.Equal("value2", data["field2"])
// 	}

// 	m = &protocol.Message{
// 		Body: []byte(`{"field1": "value1", "field2": "value2"}`),
// 	}
// 	pm = newPipeMessage(nil, m)
// 	data = pm.data()
// 	a.NotNil(data)
// 	a.IsType(map[string]interface{}{}, data)
// 	a.Contains(data, "message")
// 	a.IsType([]byte{}, data["message"])
// }
