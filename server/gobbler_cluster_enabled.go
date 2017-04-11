// +build cluster

package server

import "github.com/cosminrentea/gobbler/server/cluster"

func createCluster() (cl *cluster.Cluster) {
	var err error

	if *Config.Cluster.NodeID > 0 {
		exitIfInvalidClusterParams(*Config.Cluster.NodeID, *Config.Cluster.NodePort, *Config.Cluster.Remotes)
		logger.Info("Starting in cluster-mode")
		cl, err = cluster.New(&cluster.Config{
			ID:      *Config.Cluster.NodeID,
			Port:    *Config.Cluster.NodePort,
			Remotes: *Config.Cluster.Remotes,
		})
		if err != nil {
			logger.WithField("err", err).Fatal("Module could not be started (cluster)")
		}
	} else {
		logger.Info("Starting in standalone-mode")
	}
	return
}
