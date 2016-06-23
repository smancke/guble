package cluster

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"
)

type eventDelegate struct {
	numJoins   int
	numLeaves  int
	numUpdates int
}

func (ed *eventDelegate) NotifyJoin(node *memberlist.Node) {
	ed.numJoins++
	ed.log(node, "Cluster Node Join")
}

func (ed *eventDelegate) NotifyLeave(node *memberlist.Node) {
	ed.numLeaves++
	ed.log(node, "Cluster Node Leave")
}

func (ed *eventDelegate) NotifyUpdate(node *memberlist.Node) {
	ed.numUpdates++
	ed.log(node, "Cluster Node Update")
}

func (ed *eventDelegate) log(node *memberlist.Node, message string) {
	logger.WithFields(log.Fields{
		"node":       *node,
		"numJoins":   ed.numJoins,
		"numLeaves":  ed.numLeaves,
		"numUpdates": ed.numUpdates,
	}).Debug(message)
}
