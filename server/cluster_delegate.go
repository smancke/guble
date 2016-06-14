package server

import (
	log "github.com/Sirupsen/logrus"
)

type ClusterDelegate struct {
	messages   [][]byte
	broadcasts [][]byte
}

func (gd *ClusterDelegate) NotifyMsg(msg []byte) {
	log.WithField("message", string(msg)).Debug("NotifyMsg")
	cp := make([]byte, len(msg))
	copy(cp, msg)
	gd.messages = append(gd.messages, cp)
}

func (gd *ClusterDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	log.WithFields(log.Fields{
		"overhead": overhead,
		"limit":    limit,
	}).Debug("GetBroadcasts")
	b := gd.broadcasts
	gd.broadcasts = nil
	return b
}

func (gd *ClusterDelegate) NodeMeta(limit int) []byte {
	return nil
}

func (gd *ClusterDelegate) LocalState(join bool) []byte {
	return nil
}

func (gd *ClusterDelegate) MergeRemoteState(s []byte, join bool) {
}
