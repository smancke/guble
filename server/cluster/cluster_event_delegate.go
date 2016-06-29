package cluster

import "github.com/hashicorp/memberlist"

// ==========================================================
// memberlist.EventDelegate implementation for cluster struct
// ==========================================================

func (cluster *Cluster) NotifyJoin(node *memberlist.Node) {
	cluster.numJoins++
	cluster.log(node, "Cluster Node Join")
}

func (cluster *Cluster) NotifyLeave(node *memberlist.Node) {
	cluster.numLeaves++
	cluster.log(node, "Cluster Node Leave")
}

func (cluster *Cluster) NotifyUpdate(node *memberlist.Node) {
	cluster.numUpdates++
	cluster.log(node, "Cluster Node Update")
}
