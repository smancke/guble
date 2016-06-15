package cluster

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"
)

type ClusterEventDelegate struct {
	numJoins   int
	numLeaves  int
	numUpdates int
}

func (ced *ClusterEventDelegate) NotifyJoin(node *memberlist.Node) {
	ced.numJoins++
	ced.log(node, "Cluster Node Join")
}

func (ced *ClusterEventDelegate) NotifyLeave(node *memberlist.Node) {
	ced.numLeaves++
	ced.log(node, "Cluster Node Leave")
}

func (ced *ClusterEventDelegate) NotifyUpdate(node *memberlist.Node) {
	ced.numUpdates++
	ced.log(node, "Cluster Node Update")
}

func (ced *ClusterEventDelegate) log(node *memberlist.Node, message string) {
	logger.WithFields(log.Fields{
		"node":       *node,
		"numJoins":   ced.numJoins,
		"numLeaves":  ced.numLeaves,
		"numUpdates": ced.numUpdates,
	}).Debug(message)
}
