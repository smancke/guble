package server

import (
	log "github.com/Sirupsen/logrus"
)

type GubleDelegate struct {
	msgs       [][]byte
	broadcasts [][]byte
}

func (gd *GubleDelegate) NotifyMsg(msg []byte) {
	log.WithField("message", string(msg)).Debug("NotifyMsg")
	cp := make([]byte, len(msg))
	copy(cp, msg)
	gd.msgs = append(gd.msgs, cp)
}

func (gd *GubleDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	log.WithFields(log.Fields{
		"overhead": overhead,
		"limit":    limit,
	}).Debug("NotifyMsg")
	b := gd.broadcasts
	gd.broadcasts = nil
	return b
}

func (gd *GubleDelegate) NodeMeta(limit int) []byte {
	return nil
}

func (gd *GubleDelegate) LocalState(join bool) []byte {
	return nil
}

func (gd *GubleDelegate) MergeRemoteState(s []byte, join bool) {
}
