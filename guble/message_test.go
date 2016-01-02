package guble

import (
	assert "github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

var aNormalMessage = `/foo/bar,42,user01,phone01,id123,2015-01-01T12:00:00+01:00
{"Content-Type": "text/plain", "Correlation-Id": "7sdks723ksgqn"}
Hello World`

var aMinimalMessage = "/,42,,,,2015-01-01T12:00:00+01:00"

var aConnectedNotification = `#connected You are connected to the server.
{"ApplicationId": "phone1", "UserId": "user01", "Time": "2015-01-01T12:00:00+01:00"}`

func TestParsingANormalMessage(t *testing.T) {
	assert := assert.New(t)

	msgI, err := ParseMessage([]byte(aNormalMessage))
	assert.NoError(err)
	assert.IsType(&Message{}, msgI)
	msg := msgI.(*Message)

	assert.Equal(uint64(42), msg.Id)
	assert.Equal(Path("/foo/bar"), msg.Path)
	assert.Equal("user01", msg.PublisherUserId)
	assert.Equal("phone01", msg.PublisherApplicationId)
	assert.Equal("id123", msg.PublisherMessageId)
	assert.Equal("2015-01-01T12:00:00+01:00", msg.PublishingTime)
	assert.Equal(`{"Content-Type": "text/plain", "Correlation-Id": "7sdks723ksgqn"}`, msg.HeaderJson)

	assert.Equal("Hello World", string(msg.Body))
}

func TestSerializeANormalMessage(t *testing.T) {
	// given: a message
	msg := &Message{
		Id:                     uint64(42),
		Path:                   Path("/foo/bar"),
		PublisherUserId:        "user01",
		PublisherApplicationId: "phone01",
		PublisherMessageId:     "id123",
		PublishingTime:         "2015-01-01T12:00:00+01:00",
		HeaderJson:             `{"Content-Type": "text/plain", "Correlation-Id": "7sdks723ksgqn"}`,
		Body:                   []byte("Hello World"),
	}

	// then: the serialisation is as expected
	assert.Equal(t, aNormalMessage, string(msg.Bytes()))

	// and: the first line is as expected
	assert.Equal(t, strings.SplitN(aNormalMessage, "\n", 2)[0], msg.MetadataLine())
}

func TestSerializeAMinimalMessage(t *testing.T) {
	msg := &Message{
		Id:             uint64(42),
		Path:           Path("/"),
		PublishingTime: "2015-01-01T12:00:00+01:00",
	}

	assert.Equal(t, aMinimalMessage, string(msg.Bytes()))
}

func TestSerializeAMinimalMessageWithBody(t *testing.T) {
	msg := &Message{
		Id:             uint64(42),
		Path:           Path("/"),
		PublishingTime: "2015-01-01T12:00:00+01:00",
		Body:           []byte("Hello World"),
	}

	assert.Equal(t, aMinimalMessage+"\n\nHello World", string(msg.Bytes()))
}

func TestParsingAMinimalMessage(t *testing.T) {
	assert := assert.New(t)

	msgI, err := ParseMessage([]byte(aMinimalMessage))
	assert.NoError(err)
	assert.IsType(&Message{}, msgI)
	msg := msgI.(*Message)

	assert.Equal(uint64(42), msg.Id)
	assert.Equal(Path("/"), msg.Path)
	assert.Equal("", msg.PublisherUserId)
	assert.Equal("", msg.PublisherApplicationId)
	assert.Equal("", msg.PublisherMessageId)
	assert.Equal("2015-01-01T12:00:00+01:00", msg.PublishingTime)
	assert.Equal("", msg.HeaderJson)

	assert.Equal("", string(msg.Body))
}

func TestErrorsOnParsingMessages(t *testing.T) {
	assert := assert.New(t)

	var err error
	_, err = ParseMessage([]byte(""))
	assert.Error(err)

	// missing meta field
	_, err = ParseMessage([]byte("42,/foo/bar,user01,phone1,id123\n{}\nBla"))
	assert.Error(err)

	// id not an integer
	_, err = ParseMessage([]byte("xy42,/foo/bar,user01,phone1,id123,2015-01-01T12:00:00+01:00\n"))
	assert.Error(err)

	// path is empthy
	_, err = ParseMessage([]byte("42,,user01,phone1,id123,2015-01-01T12:00:00+01:00\n"))
	assert.Error(err)

	// Error Message without Name
	_, err = ParseMessage([]byte("!"))
	assert.Error(err)
}

func TestParsingNotificationMessage(t *testing.T) {
	assert := assert.New(t)

	msgI, err := ParseMessage([]byte(aConnectedNotification))
	assert.NoError(err)
	assert.IsType(&NotificationMessage{}, msgI)
	msg := msgI.(*NotificationMessage)

	assert.Equal(SUCCESS_CONNECTED, msg.Name)
	assert.Equal("You are connected to the server.", msg.Arg)
	assert.Equal(`{"ApplicationId": "phone1", "UserId": "user01", "Time": "2015-01-01T12:00:00+01:00"}`, msg.Json)
	assert.Equal(false, msg.IsError)
}

func TestSerializeANotificationMessage(t *testing.T) {
	msg := &NotificationMessage{
		Name:    SUCCESS_CONNECTED,
		Arg:     "You are connected to the server.",
		Json:    `{"ApplicationId": "phone1", "UserId": "user01", "Time": "2015-01-01T12:00:00+01:00"}`,
		IsError: false,
	}

	assert.Equal(t, aConnectedNotification, string(msg.Bytes()))
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

	msgI, err := ParseMessage([]byte(raw))
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
