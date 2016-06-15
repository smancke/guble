package server

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

type ClusterMessage struct {
	NodeId int
	Type   ClusterMessageType
	Body   []byte
}

func (msg *ClusterMessage) len() int {
	return int(unsafe.Sizeof(msg.Type)) + int(unsafe.Sizeof(msg.NodeId)) + len(msg.Body)
}

func (msg *ClusterMessage) EncodeMessage() (result []byte, err error) {
	bytes := make([]byte, msg.len()+5)
	enc := codec.NewEncoderBytes(&bytes, h)
	err = enc.Encode(msg)
	if err != nil {
		logger.WithField("err", err).Error("Encoding failde")
		return
	}
	return bytes, nil
}

func ParseMessage(message []byte) (*ClusterMessage, error) {
	var recvMsg ClusterMessage
	logger.WithField("bytesForDecoding", string(message)).Info("ParseMessage")

	dec := codec.NewDecoderBytes(message, h)
	err := dec.Decode(&recvMsg)
	if err != nil {
		logger.WithField("err", err).Error("Decoding failed")
		return nil, err
	}
	return &recvMsg, nil

}
