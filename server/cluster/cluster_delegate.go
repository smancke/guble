package cluster

import (
	log "github.com/Sirupsen/logrus"

	"github.com/cosminrentea/gobbler/protocol"
)

// ======================================================
// memberslist.Delegate implementation for cluster struct
// ======================================================

// NotifyMsg is invoked each time a message is received by this node of the cluster;
// it decodes and dispatches the messages.
func (cluster *Cluster) NotifyMsg(data []byte) {
	logger.WithField("msgAsBytes", data).Debug("NotifyMsg")

	cmsg := new(message)
	err := cmsg.decode(data)
	if err != nil {
		logger.WithError(err).Error("Decoding of cluster message failed")
		return
	}

	logger.WithFields(log.Fields{
		"senderNodeID": cmsg.NodeID,
		"type":         cmsg.Type,
	}).Debug("NotifyMsg: Received cluster message")

	switch cmsg.Type {
	case mtGubleMessage:
		cluster.handleGubleMessage(cmsg)
	case mtSyncPartitions:
		cluster.handleSyncPartitions(cmsg)
	case mtSyncMessage:
		cluster.handleSyncMessage(cmsg)
	case mtSyncMessageRequest:
		// cluster node is requesting to receive messages for sync
		cluster.handleSyncMessageRequest(cmsg)
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

// handles message received with type `mtGubleMessage`
func (cluster *Cluster) handleGubleMessage(cmsg *message) {
	if cluster.Router == nil {
		return
	}
	message, err := protocol.ParseMessage(cmsg.Body)
	if err != nil {
		logger.WithField("err", err).Error("Parsing of guble-message contained in cluster-message failed")
		return
	}
	cluster.Router.HandleMessage(message)
}

// handles message received with type `mtSyncPartitions`
func (cluster *Cluster) handleSyncPartitions(cmsg *message) {
	logger.WithField("message", cmsg).Debug("Received sync partitions message")

	// Decode message
	partitionsSlice := make(partitions, 0)
	// Decode data into the new slice
	err := partitionsSlice.decode(cmsg.Body)
	if err != nil {
		logger.WithError(err).Error("Error decoding partitions")
		return
	}

	logger.WithFields(log.Fields{
		"partitions": partitionsSlice,
		"nodeID":     cmsg.NodeID,
	}).Debug("Partitions received")

	// add to synchronizer
	cluster.synchronizer.sync(cmsg.NodeID, partitionsSlice)
}

func (cluster *Cluster) handleSyncMessage(cmsg *message) {
	logger.WithField("cmsg", cmsg).Debug("Handling sync message")
	err := cluster.synchronizer.syncMessage(cmsg.NodeID, cmsg.Body)
	if err != nil {
		logger.WithError(err).Error("Error synchronizing messages")
	}
}

func (cluster *Cluster) handleSyncMessageRequest(cmsg *message) {
	logger.WithField("cmsg", cmsg).Debug("Handling sync message request")
	err := cluster.synchronizer.messageRequest(cmsg.NodeID, cmsg.Body)
	if err != nil {
		logger.WithError(err).Error("Error send synchronization messages")
	}
}
