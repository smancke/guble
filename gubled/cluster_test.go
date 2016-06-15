package gubled

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/testutil"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
	"time"
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
	//actualMsg := MSG{SenderId: 1, RecID: 2, MSg: "test"}
	for i := 0; i < 10; i++ {

		body := []byte("gigi")
		actualMsg := protocol.Message{ID: 1,
			Path:          protocol.Path("/foo"),
			UserID:        "gigel",
			ApplicationID: "APPiD",
			MessageID:     "msg_id",
			Time:          time.Now().Unix(), HeaderJSON: "{}",
			Body: body}

		log.WithFields(log.Fields{
			"ID":            actualMsg.ID,
			"Path":          actualMsg.Path,
			"UserID":        actualMsg.UserID,
			"ApplicationID": actualMsg.ApplicationID,
			"Time":          actualMsg.Time,
			"hjson":         actualMsg.HeaderJSON,
			"Body":          string(actualMsg.Body),
		}).Info("Sending msg")

		bytes := make([]byte, 2000)
		enc := codec.NewEncoderBytes(&bytes, h)
		err := enc.Encode(actualMsg)
		if err != nil {
			log.WithField("err", err).Error("ENcoding failde")
			return err
		}

		//log.WithField("bytes", bytes).Info("BYTES encoded")

		var actualMsg2 protocol.Message
		dec := codec.NewDecoderBytes(bytes, h)
		err = dec.Decode(&actualMsg2)
		if err != nil {
			log.WithField("err", err).Error("rtgre")
			return err
		}
		log.WithFields(log.Fields{
			"ID":            actualMsg2.ID,
			"Path":          actualMsg2.Path,
			"UserID":        actualMsg2.UserID,
			"ApplicationID": actualMsg2.ApplicationID,
			"Time":          actualMsg2.Time,
			"hjson":         actualMsg2.HeaderJSON,
			"Body":          string(actualMsg2.Body),
		}).Info("Send1ng msg")

	}

	return nil
}

func TestEncDec(t *testing.T) {
	a := assert.New(t)
	log.SetLevel(log.DebugLevel)
	defer testutil.EnableDebugForMethod()
	err := encodeDecode()

	a.Nil(err)

}
