package server

import (
	log "github.com/Sirupsen/logrus"
)

type ClusterDelegate struct {
	messages   [][]byte
	broadcasts [][]byte
}

func (cd *ClusterDelegate) NotifyMsg(msg []byte) {
	log.WithField("msg", string(msg)).Debug("NotifyMsg")
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
