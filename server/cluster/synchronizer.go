package cluster

import (
	"strconv"
	"sync"

	log "github.com/Sirupsen/logrus"

	"github.com/smancke/guble/protocol"
	"github.com/smancke/guble/server/store"
	"github.com/ugorji/go/codec"
)

const (
	syncParititonsProcessBuffer = 10
)

type synchronizer struct {
	cluster *Cluster
	store   store.MessageStore

	// map to keep track of nodes and remote partitions and local partitions
	syncPartitions map[string]*syncPartition
	nodes          map[uint8]partitions // store the lastest info received from a node

	sync.RWMutex

	logger *log.Entry
}

func newSynchronizer(cluster *Cluster) (*synchronizer, error) {
	store, err := cluster.Router.MessageStore()
	if err != nil {
		logger.WithError(err).Error("Error retriving message store for synchronizer")
		return nil, err
	}

	return &synchronizer{
		cluster:        cluster,
		store:          store,
		syncPartitions: make(map[string]*syncPartition),

		logger: logger.WithField("module", "synchronizer"),
	}, nil
}

// add the partitions received from a node to the nodes list and start the loop
// for each partition
func (s *synchronizer) sync(nodeID uint8, partitions partitions) {
	if s.inSyncID(nodeID) {
		return
	}

	s.addNode(nodeID, partitions)
}

// inSync returns true and nodeID parsed as uint8 if current node is already
// in sync with the other node
// the cluster should not send partitions nor should accept partitions information
// from a node that is already in sync
func (s *synchronizer) inSync(nodeID string) (bool, uint8) {
	id, err := strconv.ParseUint(nodeID, 10, 8)
	if err != nil {
		logger.WithError(err).Error("Error parsing node ID")
		return false
	}
	ID := uint8(id)
	return s.inSyncID(ID), ID
}

func (s *synchronizer) inSyncID(nodeID uint8) bool {
	s.RLock()
	defer s.RUnlock()

	_, in := s.nodes[nodeID]
	return in
}

// addNode adds the node to the state with the missing partitions
func (s *synchronizer) addNode(nodeID uint8, partitions partitions) {
	s.Lock()
	defer s.Unlock()

	s.nodes[nodeID] = partitions

	for _, p := range partitions {
		sp, k := s.syncPartitions[p.Name]
		if !k {
			localMaxID, err := s.store.MaxMessageID(p.Name)
			if err != nil {
				logger.WithError(err).WithField("partition", p.Name).Error("Error retrieving max message ID for partition")
				localMaxID = 0
			}
			sp = &syncPartition{
				localMaxID: localMaxID,
				nodes:      make(map[uint8]partition, 1),
				lastID:     localMaxID,
				processC:   make(chan *syncMessage, syncParititonsProcessBuffer),
			}
		}

		sp.nodes[nodeID] = p
		s.syncPartitions[p.Name] = sp

		go sp.run()
	}
}

// keep state of fetching for a partition
type syncPartition struct {
	sync.RWMutex
	synchronizer *synchronizer

	localMaxID uint64              // max message ID in the local store
	nodes      map[uint8]partition // store nodes that have this partition and the info in does nodes
	lastID     uint64              // last fetched message ID

	// processC channel will receive the message from the cluster and store it in the
	// it's partition updating the lastID and sending a new request
	processC  chan *syncMessage
	running   bool
	runningMu sync.RWMutex
}

// start the loop that will synchronize this partition with other nodes
func (sp *syncPartition) run() {
	if sp.isRunning() {
		return
	}
	sp.setRunning(true)
	defer sp.setRunning(false)

	sp.loop()
}

// syncLoop will start to sync the partition
// Each nodes job is to ask the messages it's missing from own store.
func (sp *syncPartition) loop() {
	for {
		// send request for next message to a node
		// wait to receive the message
		// end the loop in case we are stopping the process or we finished synchronizing
	}
}

func (sp *syncPartition) isRunning() bool {
	sp.runningMu.RLock()
	defer sp.runningMu.RUnlock()

	return sp.running
}

func (sp *syncPartition) setRunning(r bool) {
	sp.runningMu.Lock()
	defer sp.runningMu.Unlock()

	sp.running = r
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
	messagePartitions, err := store.Partitions()
	if err != nil {
		logger.WithError(err).Error("Error retriving store partitions")
		return nil
	}

	partitions := make(partitions, 0, len(messagePartitions))
	for _, p := range messagePartitions {
		maxID, err := p.MaxMessageID()
		if err != nil {
			logger.
				WithError(err).
				WithField("partition", p.Name()).
				Error("Error retriving maxMessageID for partition")
			continue
		}

		partitions = append(partitions, partition{
			Name:  p.Name(),
			MaxID: maxID,
		})
	}

	return &partitions
}

type syncMessage struct {
	// Hold updated partition info
	Partition *partition
	Message   *protocol.Message
}

func (sm *syncMessage) encode() ([]byte, error) {
	return nil, nil
}

// Keep partition state
type state struct {
	partition store.MessagePartition
	nodeID    uint8  // node id from which we fetch messages for this partition
	lastID    uint64 // last fetched message ID
}
