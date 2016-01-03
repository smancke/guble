package guble

import (
	assert "github.com/stretchr/testify/assert"
	"testing"
)

var aSendCommand = `> /foo
{"meta": "data"}
Hello World`

var aSubscribeCommand = "+ /foo/bar"

func TestParsingASendCommand(t *testing.T) {
	assert := assert.New(t)

	cmd, err := ParseCmd([]byte(aSendCommand))
	assert.NoError(err)

	assert.Equal(CMD_SEND, cmd.Name)
	assert.Equal("/foo", cmd.Arg)
	assert.Equal(`{"meta": "data"}`, cmd.HeaderJson)
	assert.Equal("Hello World", string(cmd.Body))
}

func TestSerializeASendCommand(t *testing.T) {
	cmd := &Cmd{
		Name:       CMD_SEND,
		Arg:        "/foo",
		HeaderJson: `{"meta": "data"}`,
		Body:       []byte("Hello World"),
	}

	assert.Equal(t, aSendCommand, string(cmd.Bytes()))
}

func Test_Cmd_EmptyCommand_Error(t *testing.T) {
	assert := assert.New(t)
	_, err := ParseCmd([]byte{})
	assert.Error(err)
}

func TestParsingASubscribeCommand(t *testing.T) {
	assert := assert.New(t)

	cmd, err := ParseCmd([]byte(aSubscribeCommand))
	assert.NoError(err)

	assert.Equal(CMD_RECEIVE, cmd.Name)
	assert.Equal("/foo/bar", cmd.Arg)
	assert.Equal("", cmd.HeaderJson)
	assert.Nil(cmd.Body)
}

func TestSerializeASubscribeCommand(t *testing.T) {
	cmd := &Cmd{
		Name: CMD_RECEIVE,
		Arg:  "/foo/bar",
	}

	assert.Equal(t, aSubscribeCommand, string(cmd.Bytes()))
}
