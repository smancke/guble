package cluster

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEncodeDecodeNextID(t *testing.T) {
	a := assert.New(t)

	nID := NextID(10)

	msgToEncode := message{NodeID: 1, MsgType: NEXT_ID_RESPONSE, Body: nID.Bytes()}
	bytes, err := msgToEncode.encode()
	a.Nil(err)

	decodedMsg, err := decode(bytes)
	a.Nil(err)
	a.Equal(decodedMsg.Type, NEXT_ID_RESPONSE)
	a.Equal(decodedMsg.NodeID, 1)

	nextID, err := DecodeNextID(decodedMsg.Body)
	a.Nil(err)
	a.Equal(int(*nextID), 10)
}
