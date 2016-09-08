package gcm

import (
	"testing"

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

func TestPipeMessage_notification(t *testing.T) {
	a := assert.New(t)

	m := &protocol.Message{
		Body: []byte(fullFCMMessage),
	}
	pm := newPipeMessage(nil, m)
	notification := pm.notification()
	a.NotNil(notification)
	a.Equal("TEST", notification.Title)
	a.Equal("notification body", notification.Body)
	a.Equal("ic_notification_test_icon", notification.Icon)
	a.Equal("estimated_arrival", notification.ClickAction)

	m = &protocol.Message{
		Body: []byte(`{"field1": "value", "field2": "value"}`),
	}
	pm = newPipeMessage(nil, m)
	notification = pm.notification()
	a.Nil(notification)
}

func TestPipeMessage_data(t *testing.T) {
	a := assert.New(t)

	m := &protocol.Message{
		Body: []byte(fullFCMMessage),
	}
	pm := newPipeMessage(nil, m)
	data := pm.data()
	a.NotNil(data)
	a.IsType(map[string]interface{}{}, data)
	if a.Contains(data, "field1") {
		a.Equal("value1", data["field1"])
	}
	if a.Contains(data, "field2") {
		a.Equal("value2", data["field2"])
	}

	m = &protocol.Message{
		Body: []byte(`{"field1": "value1", "field2": "value2"}`),
	}
	pm = newPipeMessage(nil, m)
	data = pm.data()
	a.NotNil(data)
	a.IsType(map[string]interface{}{}, data)
	a.Contains(data, "message")
	a.IsType([]byte{}, data["message"])
}
