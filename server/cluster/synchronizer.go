package cluster

import (
	"strconv"
	"sync"

	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/store"
	"github.com/ugorji/go/codec"
)

type synchronizer struct {
	cluster    *Cluster
	store      store.MessageStore
	partitions partitions           // list of partitions at start time for current node
	nodes      map[uint8]partitions // map of nodes and their partitions

	sync.RWMutex

	running   bool
	runningMu sync.RWMutex
}

func newSynchronizer(cluster *Cluster) (*synchronizer, error) {
	store, err := cluster.Router.MessageStore()
	if err != nil {
		logger.WithError(err).Error("Error retriving message store for synchronizer")
		return nil, err
	}

	return &synchronizer{
		cluster: cluster,
		store:   store,
		nodes:   make(map[uint8]partitions),
	}, nil
}

// add the partitions received from a node to the sync list
func (s *synchronizer) sync(nodeID uint8, partitions partitions) {
	if s.inSyncID(nodeID) {
		return
	}

	s.Lock()
	defer s.Unlock()

	s.nodes[nodeID] = partitions

	// once a set of data is received start the syncloop
	go s.syncLoop()
}

// inSync returns true if current node is already in sync with the other node
// the cluster should not send partitions nor should accept partitions information
// from a node that is already in sync with
func (s *synchronizer) inSync(nodeID string) bool {
	id, err := strconv.ParseUint(nodeID, 10, 8)
	if err != nil {
		logger.WithError(err).Error("Error parsing node ID")
		return false
	}
	return s.inSyncID(uint8(id))
}

func (s *synchronizer) inSyncID(nodeID uint8) bool {
	s.RLock()
	defer s.RUnlock()

	_, in := s.nodes[nodeID]
	return in
}

// syncLoop will start to sync the partitions stored in nodes
// Each nodes job is to ask the messages it's missing from own store.
func (s *synchronizer) syncLoop() {
	if s.isRunning() {
		return
	}
	s.setRunning(true)

	// start a goroutine to sync each partition
}

func (s *synchronizer) isRunning() bool {
	s.runningMu.RLock()
	defer s.runningMu.RUnlock()

	return s.running
}

func (s *synchronizer) setRunning(r bool) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	s.running = r
}

type partition struct {
	Name  string
	MaxID uint64
}

// syncPartitions will be sent by cluster server to notify the joining server
// on the partitions they store
type partitions []partition

func (p *partitions) encode() ([]byte, error) {
	var bytes []byte
	encoder := codec.NewEncoderBytes(&bytes, h)

	err := encoder.Encode(p)
	if err != nil {
		logger.WithError(err).Error("Error encoding partitions")
		return nil, err
	}

	return bytes, nil
}

// decode will decode the bytes into the receiver `p` in our case
// Example:
// ```
// p := make(partitions, 0)
// err := p.decode(data)
// if err != nil {
// 	 ...
// }
// ```
func (p *partitions) decode(data []byte) error {
	decoder := codec.NewDecoderBytes(data, h)

	err := decoder.Decode(p)
	if err != nil {
		logger.WithError(err).Error("Error decoding partitions data")
		return err
	}

	return nil
}

func partitionsFromStore(store store.MessageStore) *partitions {
	// messagePartitions, err := store.Partitions()
	// if err != nil {
	// 	logger.WithError(err).Error("Error retriving store partitions")
	// 	return nil
	// }

	// partitions := make(partitions, 0, len(messagePartitions))
	// for _, storePartition := range messagePartitions {
	// 	maxID, err := storePartition.MaxMessageID()
	// 	if err != nil {
	// 		logger.
	// 			WithError(err).
	// 			WithField("partition", storePartition.Name).
	// 			Error("Error retriving maxMessageID for partition")
	// 		continue
	// 	}

	// 	partitions = append(partitions, partition{
	// 		Name:  storePartition.Name,
	// 		MaxID: maxID,
	// 	})
	// }

	// return &partitions
	return nil
}

type syncMessage struct {
	// Hold updated partition info
	Partition *partition
	Message   *protocol.Message
}

func (sm *syncMessage) encode() ([]byte, error) {
	return nil, nil
}
