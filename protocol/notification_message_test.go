package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsingNotificationMessage(t *testing.T) {
	assert := assert.New(t)

	msgI, err := Decode([]byte(aConnectedNotification))
	assert.NoError(err)
	assert.IsType(&NotificationMessage{}, msgI)
	msg := msgI.(*NotificationMessage)

	assert.Equal(SUCCESS_CONNECTED, msg.Name)
	assert.Equal("You are connected to the server.", msg.Arg)
	assert.Equal(`{"ApplicationId": "phone1", "UserId": "user01", "Time": "1420110000"}`, msg.Json)
	assert.Equal(false, msg.IsError)
}

func TestSerializeANotificationMessage(t *testing.T) {
	msg := &NotificationMessage{
		Name:    SUCCESS_CONNECTED,
		Arg:     "You are connected to the server.",
		Json:    `{"ApplicationId": "phone1", "UserId": "user01", "Time": "1420110000"}`,
		IsError: false,
	}

	assert.Equal(t, aConnectedNotification, string(msg.Bytes()))
}

func TestParsingErrorNotificationMessage(t *testing.T) {
	assert := assert.New(t)

	raw := "!bad-request unknown command 'sdcsd'"

	msgI, err := Decode([]byte(raw))
	assert.NoError(err)
	assert.IsType(&NotificationMessage{}, msgI)
	msg := msgI.(*NotificationMessage)

	assert.Equal("bad-request", msg.Name)
	assert.Equal("unknown command 'sdcsd'", msg.Arg)
	assert.Equal("", msg.Json)
	assert.Equal(true, msg.IsError)
}

func TestSerializeAnErrorMessage(t *testing.T) {
	msg := &NotificationMessage{
		Name:    ERROR_BAD_REQUEST,
		Arg:     "you are so bad.",
		IsError: true,
	}

	assert.Equal(t, "!"+ERROR_BAD_REQUEST+" "+"you are so bad.", string(msg.Bytes()))
}

func TestSerializeANotificationMessageWithEmptyArg(t *testing.T) {
	msg := &NotificationMessage{
		Name:    SUCCESS_SEND,
		Arg:     "",
		IsError: false,
	}

	assert.Equal(t, "#"+SUCCESS_SEND, string(msg.Bytes()))
}
