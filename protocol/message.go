package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
)

// Message is a struct that represents a message in the guble protocol, as the server sends it to the client.
type Message struct {

	// The sequenceId of the message, which is given by the
	// server an is strictly monotonically increasing at least within a root topic.
	ID uint64

	// The topic path
	Path Path

	// The user id of the message sender
	UserID string

	// The id of the sending application
	ApplicationID string

	// Filters applied to this message. The message will be sent only to the
	// routes that match the filters
	Filters map[string]string

	// The time of publishing, as Unix Timestamp date
	Time int64

	// The header line of the message (optional). If set, then it has to be a valid JSON object structure.
	HeaderJSON string

	// The message payload
	Body []byte

	NodeID uint8
}

// Metadata returns the first line of a serialized message, without the newline
func (msg *Message) Metadata() string {
	buff := &bytes.Buffer{}
	msg.writeMetadata(buff)
	return string(buff.Bytes())
}

func (msg *Message) String() string {
	return fmt.Sprintf("%d", msg.ID)
}

func (msg *Message) BodyAsString() string {
	return string(msg.Body)
}

// Bytes serializes the message into a byte slice
func (msg *Message) Bytes() []byte {
	buff := &bytes.Buffer{}

	msg.writeMetadata(buff)

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

func (msg *Message) writeMetadata(buff *bytes.Buffer) {
	buff.WriteString(string(msg.Path))
	buff.WriteString(",")
	buff.WriteString(strconv.FormatUint(msg.ID, 10))
	buff.WriteString(",")
	buff.WriteString(msg.UserID)
	buff.WriteString(",")
	buff.WriteString(msg.ApplicationID)
	buff.WriteString(",")
	buff.Write(msg.encodeFilters())
	buff.WriteString(",")
	buff.WriteString(strconv.FormatInt(msg.Time, 10))
	buff.WriteString(",")
	buff.WriteString(strconv.FormatUint(uint64(msg.NodeID), 10))
}

func (msg *Message) encodeFilters() []byte {
	if msg.Filters == nil {
		return []byte{}
	}

	data, err := json.Marshal(msg.Filters)
	if err != nil {
		log.WithError(err).WithField("filters", msg.Filters).Error("Error encoding filters")
		return []byte{}
	}

	return data
}

func (msg *Message) decodeFilters(data []byte) {
	if len(data) == 0 {
		return
	}

	msg.Filters = make(map[string]string)
	err := json.Unmarshal(data, &msg.Filters)
	if err != nil {
		log.WithError(err).WithField("data", string(data)).Error("Error decoding filters")
	}
}

func (msg *Message) SetFilter(key, value string) {
	if msg.Filters == nil {
		msg.Filters = make(map[string]string, 1)
	}
	msg.Filters[key] = value
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
	ERROR_BAD_REQUEST     = "error-bad-request"
	ERROR_INTERNAL_SERVER = "error-server-internal"
)

// NotificationMessage is a representation of a status messages or error message, sent from the server
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

// Bytes serializes the notification message into a byte slice
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

// Decode decodes a message, sent from the server to the client.
// The decoded messages can have one of the types: *Message or *NotificationMessage
func Decode(message []byte) (interface{}, error) {
	if len(message) >= 1 && (message[0] == '#' || message[0] == '!') {
		return parseNotificationMessage(message)
	}
	return ParseMessage(message)
}

func ParseMessage(message []byte) (*Message, error) {
	parts := strings.SplitN(string(message), "\n", 3)
	if len(message) == 0 {
		return nil, fmt.Errorf("empty message")
	}

	meta := strings.Split(parts[0], ",")

	if len(meta) != 7 {
		return nil, fmt.Errorf("message metadata has to have 7 fields, but was %v", parts[0])
	}

	if len(meta[0]) == 0 || meta[0][0] != '/' {
		return nil, fmt.Errorf("message has invalid topic, got %v", meta[0])
	}

	id, err := strconv.ParseUint(meta[1], 10, 0)
	if err != nil {
		return nil, fmt.Errorf("message metadata to have an integer (message-id) as second field, but was %v", meta[1])
	}

	publishingTime, err := strconv.ParseInt(meta[5], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("message metadata to have an integer (publishing time) as sixth field, but was %v", meta[5])
	}

	nodeID, err := strconv.ParseUint(meta[6], 10, 8)
	if err != nil {
		return nil, fmt.Errorf("message metadata to have an integer (nodeID) as seventh field, but was %v", meta[6])
	}

	msg := &Message{
		ID:            id,
		Path:          Path(meta[0]),
		UserID:        meta[2],
		ApplicationID: meta[3],
		Time:          publishingTime,
		NodeID:        uint8(nodeID),
	}
	msg.decodeFilters([]byte(meta[4]))

	if len(parts) >= 2 {
		msg.HeaderJSON = parts[1]
	}

	if len(parts) == 3 {
		msg.Body = []byte(parts[2])
	}

	return msg, nil
}

func parseNotificationMessage(message []byte) (*NotificationMessage, error) {
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
