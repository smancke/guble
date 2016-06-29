package cluster

import "github.com/smancke/guble/store"

// syncPartitions will be sent by cluster server to notify the joining server
// on the partitions they store
type partitions []partition

type partition struct {
	Name  string
	MaxID uint64
}

func (p *partitions) bytes() []byte {
	return nil
}

type synchornizer struct {
	cluster         *Cluster
	store           store.MessageStore
	nodesPartitions []partitions
}

func newSynchornizer(cluster *Cluster, store store.MessageStore) *synchornizer {
	return &synchornizer{cluster: cluster, store: store}
}
