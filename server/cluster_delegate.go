package server

import (
	log "github.com/Sirupsen/logrus"
)

type ClusterDelegate struct {
	messages   [][]byte
	broadcasts [][]byte
}

func (cd *ClusterDelegate) NotifyMsg(msg []byte) {
	log.WithField("msgAsBytes", msg).Debug("NotifyMsg")

	//TODO Marian decode protocol.Message

	clusterMsg, err := ParseMessage(msg)
	if err != nil {
		logger.WithField("err", err).Error("Decoding of message failed")
		return
	}
	logger.WithFields(log.Fields{
		"senderNodeID": clusterMsg.NodeId,
		"type":         clusterMsg.Type,
		"body":         string(clusterMsg.Body),
	}).Info("NotifyMsg:Received cluster message")

	cp := make([]byte, len(msg))
	copy(cp, msg)
	cd.messages = append(cd.messages, cp)
}

func (cd *ClusterDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	//TODO Cosmin uncomment or remove

	//log.WithFields(log.Fields{
	//	"overhead": overhead,
	//	"limit":    limit,
	//}).Debug("GetBroadcasts")

	b := cd.broadcasts
	cd.broadcasts = nil
	return b
}

func (cd *ClusterDelegate) NodeMeta(limit int) []byte {
	return nil
}

func (cd *ClusterDelegate) LocalState(join bool) []byte {
	return nil
}

func (cd *ClusterDelegate) MergeRemoteState(s []byte, join bool) {
}
