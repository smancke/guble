package cluster

import (
	log "github.com/Sirupsen/logrus"

	"github.com/smancke/guble/protocol"
)

// ======================================================
// memberslist.Delegate implementation for cluster struct
// ======================================================

// NotifyMsg is invoked each time a message is received by this node of the cluster; it decodes and dispatches the messages.
func (cluster *Cluster) NotifyMsg(msg []byte) {
	logger.WithField("msgAsBytes", msg).Debug("NotifyMsg")

	cmsg, err := decode(msg)
	if err != nil {
		logger.WithField("ergr", err).Error("Decoding of cluster message failed")
		return
	}

	logger.WithFields(log.Fields{
		"senderNodeID": cmsg.NodeID,
		"type":         cmsg.Type,
		"body":         string(cmsg.Body),
	}).Debug("NotifyMsg: Received cluster message")

	if cluster.Router != nil && cmsg.Type == gubleMessage {
		message, err := protocol.ParseMessage(cmsg.Body)
		if err != nil {
			logger.WithField("err", err).Error("Parsing of guble-message contained in cluster-message failed")
			return
		}
		cluster.Router.HandleMessage(message)
	}
}

func (cluster *Cluster) GetBroadcasts(overhead, limit int) [][]byte {
	b := cluster.broadcasts
	cluster.broadcasts = nil
	return b
}

func (cluster *Cluster) NodeMeta(limit int) []byte { return nil }

func (cluster *Cluster) LocalState(join bool) []byte { return nil }

func (cluster *Cluster) MergeRemoteState(s []byte, join bool) {}
