package cluster

import (
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/smancke/guble/protocol"
	"github.com/vektra/errors"
	"unsafe"
)

type messageType int

var (
	mh codec.MsgpackHandle
	h  = &mh // or mh to use msgpack
)

const (
	NEXT_ID_RESPONSE messageType = iota

	NEXT_ID_REQUEST

	// Guble protocol.Message
	MESSAGE

	STRING_BODY_MESSAGE
)

type message struct {
	NodeID int
	Type   messageType
	Body   []byte
}

func (cmsg *message) encode() ([]byte, error) {
	logger.WithField("clusterMessage", cmsg).Debug("encode")
	encodedBytes := make([]byte, cmsg.len()+5)
	encoder := codec.NewEncoderBytes(&encodedBytes, h)
	err := encoder.Encode(cmsg)
	if err != nil {
		logger.WithField("err", err).Error("Encoding failed")
		return nil, err
	}
	return encodedBytes, nil
}

func (cmsg *message) len() int {
	return int(unsafe.Sizeof(cmsg.Type)) + int(unsafe.Sizeof(cmsg.NodeID)) + len(cmsg.Body)
}

func decode(msgBytes []byte) (*message, error) {
	var cmsg message
	logger.WithField("clusterMessageBytes", string(msgBytes)).Debug("decode")

	decoder := codec.NewDecoderBytes(msgBytes, h)
	err := decoder.Decode(&cmsg)
	if err != nil {
		logger.WithField("err", err).Error("Decoding failed")
		return nil, err
	}
	return &cmsg, nil
}

// ParseMessage parses a message, sent from the server to the client.
// The parsed messages can have one of the types: *Message or *NotificationMessage or *NextID
func ParseMessage(cmsg *message) (interface{}, error) {

	switch cmsg.Type {
	case NEXT_ID_REQUEST:

		response, err := DecodeNextID(cmsg.Body)
		if err != nil {
			logger.WithField("err", err).Error("Decoding of NextId Message failed")
			return nil, err
		}
		return response, nil
	case MESSAGE:
		response, err := protocol.ParseMessage(cmsg.Body)
		if err != nil {
			logger.WithField("err", err).Error("Decoding of protocol.Message failed")
			return nil, err
		}
		return response, nil
	default:
		logger.Error("Unknown cluster message type")
		return nil, errors.New("Unkown message type")
	}

}
