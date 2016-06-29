// ==========================================================
// memberlist.EventDelegate implementation for cluster struct
// ==========================================================
package cluster

import "github.com/hashicorp/memberlist"

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
