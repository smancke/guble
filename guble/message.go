package guble

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// This scruct represents a message in the guble protocol, as the server sends to the client.
type Message struct {

	// The sequenceId of the message, which is given by the
	// server an is strictly monotonically increasing at least within a root topic.
	Id uint64

	// The topic path
	Path Path

	// The user id of the message sender
	PublisherUserId string

	// The id of the sending application
	PublisherApplicationId string

	// An id given by the sender (optional)
	PublisherMessageId string

	// The time of publishing, as iso date string
	PublishingTime string

	// The header line of the message (optional). If set, than this has to be a valid json object structure.
	HeaderJSON string

	// The message payload
	Body []byte
}

// returns the first line of a serialized message, without the newline
func (msg *Message) MetadataLine() string {
	buff := &bytes.Buffer{}
	msg.writeMetadataLine(buff)
	return string(buff.Bytes())
}

func (msg *Message) BodyAsString() string {
	return string(msg.Body)
}

// Serialize the message into a byte slice
func (msg *Message) Bytes() []byte {
	buff := &bytes.Buffer{}

	msg.writeMetadataLine(buff)

	if len(msg.HeaderJSON) > 0 || len(msg.Body) > 0 {
		buff.WriteString("\n")
	}

	if len(msg.HeaderJSON) > 0 {
		buff.WriteString(msg.HeaderJSON)
	}

	if len(msg.Body) > 0 {
		buff.WriteString("\n")
		buff.Write(msg.Body)
	}

	return buff.Bytes()
}

func (msg *Message) writeMetadataLine(buff *bytes.Buffer) {
	buff.WriteString(string(msg.Path))
	buff.WriteString(",")
	buff.WriteString(strconv.FormatUint(msg.Id, 10))
	buff.WriteString(",")
	buff.WriteString(msg.PublisherUserId)
	buff.WriteString(",")
	buff.WriteString(msg.PublisherApplicationId)
	buff.WriteString(",")
	buff.WriteString(msg.PublisherMessageId)
	buff.WriteString(",")
	buff.WriteString(msg.PublishingTime)
}

// Valid constants for the NotificationMessage.Name
const (
	SUCCESS_CONNECTED     = "connected"
	SUCCESS_SEND          = "send"
	SUCCESS_FETCH_START   = "fetch-start"
	SUCCESS_FETCH_END     = "fetch-end"
	SUCCESS_SUBSCRIBED_TO = "subscribed-to"
	SUCCESS_CANCELED      = "canceled"
	ERROR_SUBSCRIBED_TO   = "error-subscribed-to"
	ERROR_SEND            = "error-send"
	ERROR_BAD_REQUEST     = "error-bad-request"
	ERROR_INTERNAL_SERVER = "error-server-internal"
)

// Representation of a status messages or error message, send from the server
type NotificationMessage struct {

	// The name of the message
	Name string

	// The argument line, following the messageName
	Arg string

	// The optional json data supplied with the message
	Json string

	// Flag which indicates, if the notification is an error
	IsError bool
}

// Serialize the notification message into a byte slice
func (msg *NotificationMessage) Bytes() []byte {
	buff := &bytes.Buffer{}

	if msg.IsError {
		buff.WriteString("!")
	} else {
		buff.WriteString("#")
	}
	buff.WriteString(msg.Name)
	if len(msg.Arg) > 0 {
		buff.WriteString(" ")
		buff.WriteString(msg.Arg)
	}

	if len(msg.Json) > 0 {
		buff.WriteString("\n")
		buff.WriteString(msg.Json)
	}

	return buff.Bytes()
}

// The path of a topic
type Path string

func (path Path) Partition() string {
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	return strings.SplitN(string(path), "/", 2)[0]
}

// Parses a messages, send from the server to the client
// The parsed messages is the types *Message or *NotificationMessage
func ParseMessage(message []byte) (interface{}, error) {
	if len(message) >= 1 && (message[0] == '#' || message[0] == '!') {
		return parseNotificationMessage(message)
	}
	return parseMessage(message)

}

func parseMessage(message []byte) (interface{}, error) {
	parts := strings.SplitN(string(message), "\n", 3)
	if len(message) == 0 {
		return nil, fmt.Errorf("empthy message")
	}

	meta := strings.Split(parts[0], ",")
	if len(meta) != 6 {
		return nil, fmt.Errorf("message metadata has to have 6 fields, but was %v", parts[0])
	}

	if len(meta[0]) == 0 || meta[0][0] != '/' {
		return nil, fmt.Errorf("message has invalid topic, got %v", meta[0])
	}

	id, err := strconv.ParseUint(meta[1], 10, 0)
	if err != nil {
		return nil, fmt.Errorf("message metadata to have an integer id as second field, but was %v", meta[1])
	}

	msg := &Message{
		Id:                     id,
		Path:                   Path(meta[0]),
		PublisherUserId:        meta[2],
		PublisherApplicationId: meta[3],
		PublisherMessageId:     meta[4],
		PublishingTime:         meta[5],
	}

	if len(parts) >= 2 {
		msg.HeaderJSON = parts[1]
	}

	if len(parts) == 3 {
		msg.Body = []byte(parts[2])
	}

	return msg, nil
}

func parseNotificationMessage(message []byte) (interface{}, error) {

	msg := &NotificationMessage{}

	if len(message) < 2 || (message[0] != '#' && message[0] != '!') {
		return nil, fmt.Errorf("message has to start with '#' or '!' and a name, but got '%v'", message)
	}
	msg.IsError = message[0] == '!'

	parts := strings.SplitN(string(message)[1:], "\n", 2)
	firstLine := strings.SplitN(parts[0], " ", 2)

	msg.Name = firstLine[0]

	if len(firstLine) > 1 {
		msg.Arg = firstLine[1]
	}

	if len(parts) > 1 {
		msg.Json = parts[1]
	}

	return msg, nil
}
