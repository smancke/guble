// +build !cluster

package server

import "github.com/cosminrentea/gobbler/server/cluster"

func createCluster() *cluster.Cluster {
	return nil
}
