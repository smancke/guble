package cluster

import (
	"bytes"
	"github.com/hashicorp/go-msgpack/codec"
	"strconv"
	"unsafe"
)

type ClusterMessageType int

var (
	mh codec.MsgpackHandle
	h  = &mh // or mh to use msgpack
)

const (
	// NEXT_ID_REQUEST permission
	NEXT_ID_REQUEST ClusterMessageType = iota

	// NEXT_ID_RESPONSE permission
	NEXT_ID_RESPONSE

	// Guble protocol.Message
	MESSAGE

	STRING_BODY_MESSAGE
)

type NextID int

func (id *NextID) Bytes() []byte {
	buff := &bytes.Buffer{}

	buff.WriteString(strconv.Itoa(int(*id)))
	return buff.Bytes()
}

func DecodeNextID(payload []byte) (*NextID, error) {
	i, err := strconv.Atoi(string(payload))
	if err != nil {
		return nil, err
	}

	tt := NextID(int(i))

	return &tt, nil
}

type clusterMessage struct {
	NodeID int
	Type   ClusterMessageType
	Body   []byte
}

func (cmsg *clusterMessage) len() int {
	return int(unsafe.Sizeof(cmsg.Type)) + int(unsafe.Sizeof(cmsg.NodeID)) + len(cmsg.Body)
}

func (cmsg *clusterMessage) EncodeMessage() (result []byte, err error) {
	bytes := make([]byte, cmsg.len()+5)
	enc := codec.NewEncoderBytes(&bytes, h)
	err = enc.Encode(cmsg)
	if err != nil {
		logger.WithField("err", err).Error("Encoding failde")
		return
	}
	return bytes, nil
}

func ParseMessage(msg []byte) (*clusterMessage, error) {
	var recvMsg clusterMessage
	logger.WithField("bytesForDecoding", string(msg)).Info("ParseMessage")

	dec := codec.NewDecoderBytes(msg, h)
	err := dec.Decode(&recvMsg)
	if err != nil {
		logger.WithField("err", err).Error("Decoding failed")
		return nil, err
	}
	return &recvMsg, nil
}
