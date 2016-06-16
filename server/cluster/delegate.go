package cluster

import (
	log "github.com/Sirupsen/logrus"
)

type delegate struct {
	messages   [][]byte
	broadcasts [][]byte
}

func (d *delegate) NotifyMsg(msg []byte) {
	logger.WithField("msgAsBytes", msg).Debug("NotifyMsg")

	cmsg, err := decode(msg)
	if err != nil {
		logger.WithField("err", err).Error("Decoding of message failed")
		return
	}
	logger.WithFields(log.Fields{
		"senderNodeID": cmsg.NodeID,
		"type":         cmsg.Type,
		"body":         string(cmsg.Body),
	}).Info("NotifyMsg: Received cluster message")

	cp := make([]byte, len(msg))
	copy(cp, msg)
	d.messages = append(d.messages, cp)
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	//TODO Cosmin uncomment or remove

	//log.WithFields(log.Fields{
	//	"overhead": overhead,
	//	"limit":    limit,
	//}).Debug("GetBroadcasts")
	b := d.broadcasts
	d.broadcasts = nil
	return b
}

func (d *delegate) NodeMeta(limit int) []byte {
	return nil
}

func (d *delegate) LocalState(join bool) []byte {
	return nil
}

func (d *delegate) MergeRemoteState(s []byte, join bool) {
}
