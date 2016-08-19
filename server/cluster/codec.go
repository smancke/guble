package cluster

import "github.com/ugorji/go/codec"

type messageType int

var h = &codec.MsgpackHandle{}

const (
	// Guble protocol.Message
	mtGubleMessage messageType = iota

	// A node will send this message type when the body contains the partitions
	// in it's store with the max message id for each ([]partitions)
	mtSyncPartitions

	// Sent this to request a node to give us the next message so we can save it
	mtSyncNextMessageRequest

	// Sent to synchronize a message, contains the message to synchonrize along with
	// updated partition info
	mtSyncMessage

	mtStringMessage
)

type message struct {
	NodeID uint8
	Type   messageType
	Body   []byte
}

func (cmsg *message) encode() ([]byte, error) {
	logger.WithField("message", cmsg).Debug("Encoding cluster message")

	var bytes []byte
	encoder := codec.NewEncoderBytes(&bytes, h)

	err := encoder.Encode(cmsg)
	if err != nil {
		logger.WithField("err", err).Error("Encoding failed")
		return nil, err
	}

	return bytes, nil
}

func (cmsg *message) decode(cmsgBytes []byte) error {
	logger.WithField("clusterMessageBytes", string(cmsgBytes)).Debug("decode")

	decoder := codec.NewDecoderBytes(cmsgBytes, h)

	err := decoder.Decode(cmsg)
	if err != nil {
		logger.WithField("err", err).Error("Decoding failed")
		return err
	}

	return nil
}
