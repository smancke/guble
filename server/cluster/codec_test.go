package cluster

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEncodeDecodeNextID(t *testing.T) {
	a := assert.New(t)

	tt := NEXT_ID(10)

	msgToEncode := ClusterMessage{NodeID: 1, Type: NEXT_ID_RESPONSE, Body: tt.Bytes()}
	bytes, err := msgToEncode.EncodeMessage()
	a.Nil(err)

	decodedMsg, err := ParseMessage(bytes)
	a.Nil(err)
	a.Equal(decodedMsg.Type, NEXT_ID_RESPONSE)
	a.Equal(decodedMsg.NodeID, 1)

	nextID, err := DecodeNextID(decodedMsg.Body)
	a.Nil(err)
	a.Equal(int(*nextID), 10)

}
