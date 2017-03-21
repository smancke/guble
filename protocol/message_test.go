package protocol

import (
	"strings"
	"testing"

	"time"

	"github.com/stretchr/testify/assert"
)

var (
	aNormalMessage = `/foo/bar,42,user01,phone01,{"user":"user01"},1420110000,1420110000,1
{"Content-Type": "text/plain", "Correlation-Id": "7sdks723ksgqn"}
Hello World`

	aNormalMessageNoExpires = `/foo/bar,42,user01,phone01,{"user":"user01"},0,1420110000,1
{"Content-Type": "text/plain", "Correlation-Id": "7sdks723ksgqn"}
Hello World`

	aMinimalMessage = "/,42,,,,0,1420110000,0"

	aConnectedNotification = `#connected You are connected to the server.
{"ApplicationId": "phone1", "UserId": "user01", "Time": "1420110000"}`

	// 2015-01-01T12:00:00+01:00 is equal to  1420110000
	unixTime, _ = time.Parse(time.RFC3339, "2015-01-01T12:00:00+01:00")
)

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
	assert.Equal(t, aNormalMessageNoExpires, string(msg.Encode()))
	assert.Equal(t, "42: Hello World", msg.String())

	// and: the first line is as expected
	assert.Equal(t, strings.SplitN(aNormalMessageNoExpires, "\n", 2)[0], msg.Metadata())
}

func TestSerializeANormalMessageWithExpires(t *testing.T) {
	// given: a message
	msg := &Message{
		ID:            uint64(42),
		Path:          Path("/foo/bar"),
		UserID:        "user01",
		ApplicationID: "phone01",
		Filters:       map[string]string{"user": "user01"},
		Expires:       1420110000,
		Time:          unixTime.Unix(),
		NodeID:        1,
		HeaderJSON:    `{"Content-Type": "text/plain", "Correlation-Id": "7sdks723ksgqn"}`,
		Body:          []byte("Hello World"),
	}

	// then: the serialisation is as expected
	assert.Equal(t, aNormalMessage, string(msg.Encode()))
	assert.Equal(t, "42: Hello World", msg.String())

	// and: the first line is as expected
	assert.Equal(t, strings.SplitN(aNormalMessage, "\n", 2)[0], msg.Metadata())
}

func TestSerializeAMinimalMessage(t *testing.T) {
	msg := &Message{
		ID:   uint64(42),
		Path: Path("/"),
		Time: unixTime.Unix(),
	}

	assert.Equal(t, aMinimalMessage, string(msg.Encode()))
}

func TestSerializeAMinimalMessageWithBody(t *testing.T) {
	msg := &Message{
		ID:   uint64(42),
		Path: Path("/"),
		Time: unixTime.Unix(),
		Body: []byte("Hello World"),
	}

	assert.Equal(t, aMinimalMessage+"\n\nHello World", string(msg.Encode()))
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
	assert.Nil(msg.Filters)
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

func Test_Message_getPartitionFromTopic(t *testing.T) {
	a := assert.New(t)
	a.Equal("foo", Path("/foo/bar/bazz").Partition())
	a.Equal("foo", Path("/foo").Partition())
	a.Equal("", Path("/").Partition())
	a.Equal("", Path("").Partition())
}

func TestMessage_Filters(t *testing.T) {
	a := assert.New(t)

	msg := &Message{}
	msg.SetFilter("user", "user01")
	msg.SetFilter("device_id", "ID_DEVICE")

	a.NotNil(msg.Filters)
	a.Equal(msg.Filters["user"], "user01")
	a.Equal(msg.Filters["device_id"], "ID_DEVICE")

	a.JSONEq(`{"user": "user01","device_id":"ID_DEVICE"}`, string(msg.encodeFilters()))
}

func TestMessage_decodeFilters(t *testing.T) {
	a := assert.New(t)

	msg := &Message{}

	filters := []byte(`{"user": "user01","device_id":"ID_DEVICE"}`)
	msg.decodeFilters(filters)

	a.NotNil(msg.Filters)
	a.Contains(msg.Filters, "user")
	a.Contains(msg.Filters, "device_id")

	a.Equal(msg.Filters["user"], "user01")
	a.Equal(msg.Filters["device_id"], "ID_DEVICE")
}

func TestMessage_IsExpired(t *testing.T) {
	a := assert.New(t)

	n := time.Now().UTC()
	loc1, err := time.LoadLocation("Europe/Berlin")
	a.NoError(err)

	loc2, err := time.LoadLocation("Europe/Bucharest")
	a.NoError(err)

	cases := []struct {
		expires time.Time
		result  bool
	}{
		{n.AddDate(0, 0, 1), false},
		{n.AddDate(0, 0, -1), true},
		{n.AddDate(0, 0, 1).In(loc1), false},
		{n.AddDate(0, 0, 1).In(loc2), false},
		{n.AddDate(0, 0, -1).In(loc1), true},
		{n.AddDate(0, 0, -1).In(loc2), true},
		{n.Add(1 * time.Minute).In(loc1), false},
		{n.Add(1 * time.Minute).In(loc2), false},
		{n.Add(-1 * time.Minute).In(loc1), true},
		{n.Add(-1 * time.Minute).In(loc2), true},
	}

	for i, c := range cases {
		a.Equal(c.result, (&Message{Expires: c.expires.Unix()}).IsExpired(), "Failed IsExpired case: %d", i)
	}

}

func TestMessage_IsExpired_WithNilExpires(t *testing.T) {
	a := assert.New(t)

	a.Equal(false, (&Message{}).IsExpired())

}
