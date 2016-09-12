package protocol

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var aSendCommand = `> /foo
{"meta": "data"}
Hello World`

var aSubscribeCommand = "+ /foo/bar"

func TestParsingASendCommand(t *testing.T) {
	a := assert.New(t)

	cmd, err := ParseCmd([]byte(aSendCommand))
	a.NoError(err)

	a.Equal(CmdSend, cmd.Name)
	a.Equal("/foo", cmd.Arg)
	a.Equal(`{"meta": "data"}`, cmd.HeaderJSON)
	a.Equal("Hello World", string(cmd.Body))
}

func TestSerializeASendCommand(t *testing.T) {
	cmd := &Cmd{
		Name:       CmdSend,
		Arg:        "/foo",
		HeaderJSON: `{"meta": "data"}`,
		Body:       []byte("Hello World"),
	}

	assert.Equal(t, aSendCommand, string(cmd.Bytes()))
}

func Test_Cmd_EmptyCommand_Error(t *testing.T) {
	a := assert.New(t)
	_, err := ParseCmd([]byte{})
	a.Error(err)
}

func TestParsingASubscribeCommand(t *testing.T) {
	a := assert.New(t)

	cmd, err := ParseCmd([]byte(aSubscribeCommand))
	a.NoError(err)

	a.Equal(CmdReceive, cmd.Name)
	a.Equal("/foo/bar", cmd.Arg)
	a.Equal("", cmd.HeaderJSON)
	a.Nil(cmd.Body)
}

func TestSerializeASubscribeCommand(t *testing.T) {
	cmd := &Cmd{
		Name: CmdReceive,
		Arg:  "/foo/bar",
	}

	assert.Equal(t, aSubscribeCommand, string(cmd.Bytes()))
}
