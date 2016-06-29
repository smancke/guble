// =============================================================
// memberlist.ConflictDelegate implementation for cluster struct
// =============================================================
package cluster

import (
	log "github.com/Sirupsen/logrus"

	"github.com/hashicorp/memberlist"
)

func (cluster *Cluster) NotifyConflict(existing, other *memberlist.Node) {
	logger.WithFields(log.Fields{
		"existing": *existing,
		"other":    *other,
	}).Panic("NotifyConflict")
}
