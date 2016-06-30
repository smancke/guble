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

func partitionsFromStore(store store.MessageStore) *partitions {
	messagePartitions, err := store.Partitions()
	if err != nil {
		logger.WithError(err).Error("Error retriving store partitions")
		return nil
	}

	partitions := make(partitions, 0, len(messagePartitions))
	for _, storePartition := range messagePartitions {
		maxID, err := storePartition.MaxMessageID()
		if err != nil {
			logger.
				WithError(err).
				WithField("partition", storePartition.Name).
				Error("Error retriving maxMessageID for partition")
			continue
		}

		partitions = append(partitions, partition{
			Name:  storePartition.Name,
			MaxID: maxID,
		})
	}

	return &partitions

}

type synchornizer struct {
	cluster         *Cluster
	store           store.MessageStore
	nodesPartitions []partitions
}

func newSynchornizer(cluster *Cluster, store store.MessageStore) *synchornizer {
	return &synchornizer{cluster: cluster, store: store}
}
