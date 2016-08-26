package cluster

import (
	log "github.com/Sirupsen/logrus"

	"github.com/ugorji/go/codec"
)

type messageType int

var h = &codec.MsgpackHandle{}

const (
	// Guble protocol.Message
	mtGubleMessage messageType = iota

	// A node will send this message type when the body contains the partitions
	// in it's store with the max message id for each ([]partitions)
	mtSyncPartitions

	// Sent this to request a node to give us the next message so we can save it
	mtSyncMessageRequest

	// Sent to synchronize a message, contains the message to synchonrize along with
	// updated partition info
	mtSyncMessage

	mtStringMessage
)

type encoder interface {
	encode() ([]byte, error)
}

type decoder interface {
	decode(data []byte) error
}

type message struct {
	NodeID uint8
	Type   messageType
	Body   []byte
}

func (cmsg *message) encode() ([]byte, error) {
	logger.WithFields(log.Fields{
		"nodeID": cmsg.NodeID,
		"type":   cmsg.Type,
		"body":   string(cmsg.Body),
	}).Debug("Encoding cluster message")
	return encode(cmsg)
}

func (cmsg *message) decode(data []byte) error {
	logger.WithField("data", string(data)).Debug("decode")
	return decode(cmsg, data)
}

func encode(entity interface{}) ([]byte, error) {
	logger.WithField("entity", entity).Debug("Encoding")

	var bytes []byte
	encoder := codec.NewEncoderBytes(&bytes, h)

	err := encoder.Encode(entity)
	if err != nil {
		logger.WithField("err", err).Error("Encoding failed")
		return nil, err
	}

	return bytes, nil
}

func decode(o interface{}, data []byte) error {
	logger.WithField("data", string(data)).Debug("Decoding")

	decoder := codec.NewDecoderBytes(data, h)

	err := decoder.Decode(o)
	if err != nil {
		logger.WithField("err", err).Error("Decoding failed")
		return err
	}

	return nil
}
