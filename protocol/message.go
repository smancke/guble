package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

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

	// Expires field specifies until when the message is valid to be processed
	// If this field is set and the message is expired the connectors should
	// consider the message as processed and log the action
	//
	// The field value is a Unix Timestamp in Seconds
	Expires int64

	// The time of publishing, as Unix Timestamp date
	Time int64

	// The header line of the message (optional). If set, then it has to be a valid JSON object structure.
	HeaderJSON string

	// The message payload
	Body []byte

	// Used in cluster mode to identify a guble node
	NodeID uint8
}

type MessageDeliveryCallback func(*Message)

// Metadata returns the first line of a serialized message, without the newline
func (m *Message) Metadata() string {
	buff := &bytes.Buffer{}
	m.writeMetadata(buff)
	return string(buff.Bytes())
}

func (m *Message) String() string {
	return fmt.Sprintf("%d: %s", m.ID, string(m.Body))
}

// Encode serializes the message into a byte slice
func (m *Message) Encode() []byte {
	buff := &bytes.Buffer{}

	m.writeMetadata(buff)

	if len(m.HeaderJSON) > 0 || len(m.Body) > 0 {
		buff.WriteString("\n")
	}

	if len(m.HeaderJSON) > 0 {
		buff.WriteString(m.HeaderJSON)
	}

	if len(m.Body) > 0 {
		buff.WriteString("\n")
		buff.Write(m.Body)
	}

	return buff.Bytes()
}

func (m *Message) writeMetadata(buff *bytes.Buffer) {
	buff.WriteString(string(m.Path))
	buff.WriteByte(',')

	buff.WriteString(strconv.FormatUint(m.ID, 10))
	buff.WriteByte(',')

	buff.WriteString(m.UserID)
	buff.WriteByte(',')

	buff.WriteString(m.ApplicationID)
	buff.WriteByte(',')

	buff.Write(m.encodeFilters())
	buff.WriteByte(',')

	buff.WriteString(strconv.FormatInt(m.Expires, 10))
	buff.WriteByte(',')

	buff.WriteString(strconv.FormatInt(m.Time, 10))
	buff.WriteByte(',')

	buff.WriteString(strconv.FormatUint(uint64(m.NodeID), 10))
}

func (m *Message) encodeFilters() []byte {
	if m.Filters == nil {
		return []byte{}
	}
	data, err := json.Marshal(m.Filters)
	if err != nil {
		log.WithError(err).WithField("filters", m.Filters).Error("Error encoding filters")
		return []byte{}
	}
	return data
}

func (m *Message) decodeFilters(data []byte) {
	if len(data) == 0 {
		return
	}
	m.Filters = make(map[string]string)
	err := json.Unmarshal(data, &m.Filters)
	if err != nil {
		log.WithError(err).WithField("data", string(data)).Error("Error decoding filters")
	}

}

func (m *Message) SetFilter(key, value string) {
	if m.Filters == nil {
		m.Filters = make(map[string]string, 1)
	}
	m.Filters[key] = value
}

// IsExpired returns true if the message `Expires` field is set and the current time
// has pasted the `Expires` time
func (m *Message) IsExpired() bool {
	if m.Expires == 0 {
		return false
	}
	return m.Expires != 0 && m.Expires < time.Now().Unix()
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

	if len(meta) != 8 {
		return nil, fmt.Errorf("message metadata has to have 7 fields, but was %v", parts[0])
	}

	if len(meta[0]) == 0 || meta[0][0] != '/' {
		return nil, fmt.Errorf("message has invalid topic, got %v", meta[0])
	}

	id, err := strconv.ParseUint(meta[1], 10, 0)
	if err != nil {
		return nil, fmt.Errorf("message metadata to have an integer (message-id) as second field, but was %v", meta[1])
	}

	var expiresTime int64
	if meta[5] != "" {
		var err error
		expiresTime, err = strconv.ParseInt(meta[5], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("message metadata expected to have a  time string (expiration time) as sixth field, but was %v: %s", meta[5], err.Error())
		}
	}

	publishingTime, err := strconv.ParseInt(meta[6], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("message metadata to have an integer (publishing time) as seventh field, but was %v:", meta[6])
	}

	nodeID, err := strconv.ParseUint(meta[7], 10, 8)
	if err != nil {
		return nil, fmt.Errorf("message metadata to have an integer (nodeID) as eighth field, but was %v", meta[7])
	}

	m := &Message{
		ID:            id,
		Path:          Path(meta[0]),
		UserID:        meta[2],
		ApplicationID: meta[3],
		Expires:       expiresTime,
		Time:          publishingTime,
		NodeID:        uint8(nodeID),
	}
	m.decodeFilters([]byte(meta[4]))

	if len(parts) >= 2 {
		m.HeaderJSON = parts[1]
	}

	if len(parts) == 3 {
		m.Body = []byte(parts[2])
	}

	return m, nil
}
