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

	//MESSAGE
	MESSAGE
	//
	STRING_BODY_MESSAGE
)

type NEXT_ID int

func (id *NEXT_ID) Bytes() []byte {
	buff := &bytes.Buffer{}

	buff.WriteString(strconv.Itoa(int(*id)))
	return buff.Bytes()
}

func DecodeNextID(payload []byte) (*NEXT_ID, error) {
	i, err := strconv.Atoi(string(payload))
	if err != nil {
		return nil, err
	}

	tt := NEXT_ID(int(i))

	return &tt, nil
}

type ClusterMessage struct {
	NodeID int
	Type   ClusterMessageType
	Body   []byte
}

func (cmsg *ClusterMessage) len() int {
	return int(unsafe.Sizeof(cmsg.Type)) + int(unsafe.Sizeof(cmsg.NodeID)) + len(cmsg.Body)
}

func (cmsg *ClusterMessage) EncodeMessage() (result []byte, err error) {
	bytes := make([]byte, cmsg.len()+5)
	enc := codec.NewEncoderBytes(&bytes, h)
	err = enc.Encode(cmsg)
	if err != nil {
		logger.WithField("err", err).Error("Encoding failde")
		return
	}
	return bytes, nil
}

func ParseMessage(msg []byte) (*ClusterMessage, error) {
	var recvMsg ClusterMessage
	logger.WithField("bytesForDecoding", string(msg)).Info("ParseMessage")

	dec := codec.NewDecoderBytes(msg, h)
	err := dec.Decode(&recvMsg)
	if err != nil {
		logger.WithField("err", err).Error("Decoding failed")
		return nil, err
	}
	return &recvMsg, nil
}
