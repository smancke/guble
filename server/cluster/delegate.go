package cluster

import (
	log "github.com/Sirupsen/logrus"
)

type Delegate struct {
	messages   [][]byte
	broadcasts [][]byte
}

func (d *Delegate) NotifyMsg(msg []byte) {
	log.WithField("msgAsBytes", msg).Debug("NotifyMsg")

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

func (d *Delegate) GetBroadcasts(overhead, limit int) [][]byte {
	//TODO Cosmin uncomment or remove

	//log.WithFields(log.Fields{
	//	"overhead": overhead,
	//	"limit":    limit,
	//}).Debug("GetBroadcasts")
	b := d.broadcasts
	d.broadcasts = nil
	return b
}

func (d *Delegate) NodeMeta(limit int) []byte {
	return nil
}

func (d *Delegate) LocalState(join bool) []byte {
	return nil
}

func (d *Delegate) MergeRemoteState(s []byte, join bool) {
}
