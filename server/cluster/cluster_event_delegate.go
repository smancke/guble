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
	logger.WithField("node", node.Name).Debug("Sending partitions info")

	// Send message partitions to the new node
	store, err := cluster.Router.MessageStore()
	if err != nil {
		logger.WithError(err).Error("Error retriving message store to get partitions")
		return
	}

	partitions := partitionsFromStore(store)

	// sending partitions
	cmsg := cluster.newMessage(syncPartitions, partitions.bytes())

	// send message to node
	err = cluster.sendMessageToNode(node, cmsg)
	if err != nil {
		logger.WithField("node", node.Name).WithError(err).Error("Error sending partitions info to node")
	}
}

func (cluster *Cluster) getSynchornizer() (*synchornizer, error) {
	if cluster.synchornizer == nil {
		store, err := cluster.Router.MessageStore()
		if err != nil {
			logger.WithError(err).Error("Error retriving message store for synchronizer")
			return nil, err
		}
		cluster.synchornizer = newSynchornizer(cluster, store)
	}

	return cluster.synchornizer, nil
}
