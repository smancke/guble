package protocol

import (
	"bytes"
	"fmt"
	"strings"
)

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
