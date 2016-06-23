package cluster

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEncodeDecodeNextID(t *testing.T) {
	a := assert.New(t)

	nID := NextID(10)

	msgToEncode := message{NodeID: 1, Type: nextIdResponse, Body: nID.Bytes()}
	bytes, err := msgToEncode.encode()
	a.Nil(err)

	decodedMsg, err := decode(bytes)
	a.Nil(err)
	a.Equal(decodedMsg.Type, nextIdResponse)
	a.Equal(decodedMsg.NodeID, 1)

	nextID, err := decodeNextID(decodedMsg.Body)
	a.Nil(err)
	a.Equal(int(*nextID), 10)
}
