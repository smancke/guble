package gubled

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

var (
	mh codec.MsgpackHandle
)

var (
	r io.Reader
	w io.Writer
	b []byte
	h = &mh // or mh to use msgpack
)

type MSG struct {
	SenderId int
	RecID    int
	MSg      string
}

func encodeDecode() error {
	actualMsg := MSG{SenderId: 1, RecID: 2, MSg: "test"}
	log.WithFields(log.Fields{
		"senderId":   actualMsg.SenderId,
		"receiverID": actualMsg.RecID,
		"msg":        actualMsg.MSg,
	}).Info("Sending msg")

	bytes := make([]byte, 200)
	enc := codec.NewEncoderBytes(&bytes, h)
	err := enc.Encode(actualMsg)
	if err != nil {
		log.WithField("err", err).Error("ENcoding failde")
		return err
	}

	log.WithField("bytes", bytes).Info("BYTES encoded")

	var actualMsg2 MSG
	dec := codec.NewDecoderBytes(bytes, h)
	err = dec.Decode(&actualMsg2)
	if err != nil {
		log.WithField("err", err).Error("rtgre")
		return err
	}

	log.WithFields(log.Fields{
		"senderId":   actualMsg2.SenderId,
		"receiverID": actualMsg2.RecID,
		"msg":        actualMsg2.MSg,
	}).Info("Decoded")

	return nil
}

func TestEncDec(t *testing.T) {
	a := assert.New(t)
	log.SetLevel(log.DebugLevel)
	defer testutil.EnableDebugForMethod()
	err := encodeDecode()

	a.Nil(err)

}
