package cluster

import (
	log "github.com/Sirupsen/logrus"

	"github.com/hashicorp/memberlist"
)

// ==========================================================
// memberlist.EventDelegate implementation for cluster struct
// ==========================================================

func (cluster *Cluster) NotifyJoin(node *memberlist.Node) {
	cluster.numJoins++
	cluster.eventLog(node, "Cluster Node Join")

	cluster.sendPartitions(node)
}

func (cluster *Cluster) NotifyLeave(node *memberlist.Node) {
	cluster.numLeaves++
	cluster.eventLog(node, "Cluster Node Leave")
}

func (cluster *Cluster) NotifyUpdate(node *memberlist.Node) {
	cluster.numUpdates++
	cluster.eventLog(node, "Cluster Node Update")
}

func (cluster *Cluster) eventLog(node *memberlist.Node, message string) {
	logger.WithFields(log.Fields{
		"node":       node.Name,
		"numJoins":   cluster.numJoins,
		"numLeaves":  cluster.numLeaves,
		"numUpdates": cluster.numUpdates,
	}).Debug(message)
}

func (cluster *Cluster) sendPartitions(node *memberlist.Node) {
	if _, inSync := cluster.synchronizer.inSync(node.Name); inSync {
		logger.WithField("node", node.Name).Debug("Already in sync with node")
		return
	}

	logger.WithField("node", node.Name).Debug("Sending partitions info")

	// Send message partitions to the new node
	store, err := cluster.Router.MessageStore()
	if err != nil {
		logger.WithError(err).Error("Error retriving message store to get partitions")
		return
	}

	partitionsSlice := partitionsFromStore(store)

	// sending partitions
	data, err := partitionsSlice.encode()
	if err != nil {
		logger.WithError(err).Error("Error encoding partitions")
		return
	}
	cmsg := cluster.newMessage(mtSyncPartitions, data)

	// send message to node
	err = cluster.sendMessageToNode(node, cmsg)
	if err != nil {
		logger.WithField("node", node.Name).WithError(err).Error("Error sending partitions info to node")
	}
}
