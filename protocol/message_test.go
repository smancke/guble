package protocol

import (
	"strings"
	"testing"

	"time"

	"github.com/stretchr/testify/assert"
)

var aNormalMessage = `/foo/bar,42,user01,phone01,{"user":"user01"},1420110000,1
{"Content-Type": "text/plain", "Correlation-Id": "7sdks723ksgqn"}
Hello World`

var aMinimalMessage = "/,42,,,,1420110000,0"

var aConnectedNotification = `#connected You are connected to the server.
{"ApplicationId": "phone1", "UserId": "user01", "Time": "1420110000"}`

// 2015-01-01T12:00:00+01:00 is equal to  1420110000
var unixTime, _ = time.Parse(time.RFC3339, "2015-01-01T12:00:00+01:00")

func TestParsingANormalMessage(t *testing.T) {
	assert := assert.New(t)

	msgI, err := Decode([]byte(aNormalMessage))
	assert.NoError(err)
	assert.IsType(&Message{}, msgI)
	msg := msgI.(*Message)

	assert.Equal(uint64(42), msg.ID)
	assert.Equal(Path("/foo/bar"), msg.Path)
	assert.Equal("user01", msg.UserID)
	assert.Equal("phone01", msg.ApplicationID)
	assert.Equal(map[string]string{"user": "user01"}, msg.Filters)
	assert.Equal(unixTime.Unix(), msg.Time)
	assert.Equal(uint8(1), msg.NodeID)
	assert.Equal(`{"Content-Type": "text/plain", "Correlation-Id": "7sdks723ksgqn"}`, msg.HeaderJSON)
	assert.Equal("Hello World", string(msg.Body))
}

func TestSerializeANormalMessage(t *testing.T) {
	// given: a message
	msg := &Message{
		ID:            uint64(42),
		Path:          Path("/foo/bar"),
		UserID:        "user01",
		ApplicationID: "phone01",
		Filters:       map[string]string{"user": "user01"},
		Time:          unixTime.Unix(),
		NodeID:        1,
		HeaderJSON:    `{"Content-Type": "text/plain", "Correlation-Id": "7sdks723ksgqn"}`,
		Body:          []byte("Hello World"),
	}

	// then: the serialisation is as expected
	assert.Equal(t, aNormalMessage, string(msg.Bytes()))
	assert.Equal(t, "Hello World", msg.BodyAsString())

	// and: the first line is as expected
	assert.Equal(t, strings.SplitN(aNormalMessage, "\n", 2)[0], msg.Metadata())
}

func TestSerializeAMinimalMessage(t *testing.T) {
	msg := &Message{
		ID:   uint64(42),
		Path: Path("/"),
		Time: unixTime.Unix(),
	}

	assert.Equal(t, aMinimalMessage, string(msg.Bytes()))
}

func TestSerializeAMinimalMessageWithBody(t *testing.T) {
	msg := &Message{
		ID:   uint64(42),
		Path: Path("/"),
		Time: unixTime.Unix(),
		Body: []byte("Hello World"),
	}

	assert.Equal(t, aMinimalMessage+"\n\nHello World", string(msg.Bytes()))
}

func TestParsingAMinimalMessage(t *testing.T) {
	assert := assert.New(t)

	msgI, err := Decode([]byte(aMinimalMessage))
	assert.NoError(err)
	assert.IsType(&Message{}, msgI)
	msg := msgI.(*Message)

	assert.Equal(uint64(42), msg.ID)
	assert.Equal(Path("/"), msg.Path)
	assert.Equal("", msg.UserID)
	assert.Equal("", msg.ApplicationID)
	assert.Equal(map[string]string{}, msg.Filters)
	assert.Equal(unixTime.Unix(), msg.Time)
	assert.Equal("", msg.HeaderJSON)

	assert.Equal("", string(msg.Body))
}

func TestErrorsOnParsingMessages(t *testing.T) {
	assert := assert.New(t)

	var err error
	_, err = Decode([]byte(""))
	assert.Error(err)

	// missing meta field
	_, err = Decode([]byte("42,/foo/bar,user01,phone1,id123\n{}\nBla"))
	assert.Error(err)

	// id not an integer
	_, err = Decode([]byte("xy42,/foo/bar,user01,phone1,id123,1420110000\n"))
	assert.Error(err)

	// path is empty
	_, err = Decode([]byte("42,,user01,phone1,id123,1420110000\n"))
	assert.Error(err)

	// Error Message without Name
	_, err = Decode([]byte("!"))
	assert.Error(err)
}

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

func Test_Message_getPartitionFromTopic(t *testing.T) {
	a := assert.New(t)
	a.Equal("foo", Path("/foo/bar/bazz").Partition())
	a.Equal("foo", Path("/foo").Partition())
	a.Equal("", Path("/").Partition())
	a.Equal("", Path("").Partition())
}
