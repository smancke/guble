package guble

import (
	"bytes"
	"fmt"
	"strings"
)

// Valid command names
const (
	CMD_SEND        = "send"
	CMD_SUBSCRIBE   = "subscribe"
	CMD_UNSUBSCRIBE = "unsubscribe"
)

// Representation of a command, which the client sends to the server
type Cmd struct {

	// The name of the command
	Name string

	// The argument line, following the commandName
	Arg string

	// The header line, if the command has one
	HeaderJson string

	// The command payload, if the command has such
	Body []byte
}

// Parses a Command
func ParseCmd(message []byte) (*Cmd, error) {
	msg := &Cmd{}

	if len(message) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	parts := strings.SplitN(string(message), "\n", 3)
	firstLine := strings.SplitN(parts[0], " ", 2)

	msg.Name = firstLine[0]

	if len(firstLine) > 1 {
		msg.Arg = firstLine[1]
	}

	if len(parts) > 1 {
		msg.HeaderJson = parts[1]
	}

	if len(parts) > 2 {
		msg.Body = []byte(parts[2])
	}

	return msg, nil
}

// Serialize the the command into a byte slice
func (cmd *Cmd) Bytes() []byte {
	buff := &bytes.Buffer{}
	buff.WriteString(cmd.Name)
	buff.WriteString(" ")
	buff.WriteString(cmd.Arg)

	if len(cmd.HeaderJson) > 0 || len(cmd.Body) > 0 {
		buff.WriteString("\n")
	}

	if len(cmd.HeaderJson) > 0 {
		buff.WriteString(cmd.HeaderJson)
	}

	if len(cmd.Body) > 0 {
		buff.WriteString("\n")
		buff.Write(cmd.Body)
	}

	return buff.Bytes()
}
