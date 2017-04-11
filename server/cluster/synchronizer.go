package cluster

import (
	"errors"
	"math"
	"strconv"
	"sync"

	log "github.com/Sirupsen/logrus"

	"github.com/cosminrentea/gobbler/server/store"
	"github.com/ugorji/go/codec"
)

const (
	syncPartitionsProcessBuffer = 100
)

var (
	ErrNodeNotInSync = errors.New("Node not found in syncPartitions list.")

	ErrMissingSyncPartition = errors.New("Missing sync partition")
)

type synchronizer struct {
	cluster *Cluster
	store   store.MessageStore

	// map to keep track of nodes and remote partitions and local partitions
	syncPartitions map[string]*syncPartition
	nodes          map[uint8]partitions // store the lastest info received from a node

	sync.RWMutex

	logger *log.Entry
	stopC  chan struct{}
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
		nodes:          make(map[uint8]partitions),

		logger: logger.WithField("module", "synchronizer"),
		stopC:  make(chan struct{}),
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

// inSync returns nodeID and a boolean value specifying if this node is already in sync
// returns 0 as nodeID and false if the node cannot be parsed
// the cluster should not send partitions nor should accept partitions information
// from a node that is already in sync
func (s *synchronizer) inSync(nodeID string) (uint8, bool) {
	id, err := strconv.ParseUint(nodeID, 10, 8)
	if err != nil {
		logger.WithError(err).Error("Error parsing node ID")
		return 0, false
	}
	ID := uint8(id)
	return ID, s.inSyncID(ID)
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
		sp, exists := s.syncPartitions[p.Name]
		if !exists {
			localPartition, err := s.store.Partition(p.Name)
			if err != nil {
				logger.WithError(err).WithField("partition", p.Name).Error("Error retrieving local partition")
				return
			}
			localMaxID := localPartition.MaxMessageID()

			sp = &syncPartition{
				synchronizer:    s,
				localPartition:  localPartition,
				localStartMaxID: localMaxID,
				nodes:           make(map[uint8]partition, 1),
				lastID:          localMaxID,
				processC:        make(chan *syncMessage, syncPartitionsProcessBuffer),
			}
		}

		sp.nodes[nodeID] = p
		s.syncPartitions[p.Name] = sp

		go sp.run()
	}
}

func (s *synchronizer) messageRequest(nodeID uint8, data []byte) error {
	smr := &syncMessageRequest{}
	err := smr.decode(data)
	if err != nil {
		return err
	}

	// start goroutine that will fetch messages from store and send them to the node
	go s.requestLoop(nodeID, smr)
	return nil
}

// requestLoop handles sending messages fetched from the store to the node
// that made the request a message sent from here will be received by the syncMessage
// method on the other node
func (s *synchronizer) requestLoop(nodeID uint8, smr *syncMessageRequest) {
	s.logger.WithFields(log.Fields{
		"requestNodeID":      nodeID,
		"syncMessageRequest": smr,
	}).Debug("Sending requested messages")

	req := &store.FetchRequest{
		Partition: smr.Partition,
		StartID:   smr.StartID,
		EndID:     smr.EndID,
		Direction: 1,
		MessageC:  make(chan *store.FetchedMessage, 10),
		ErrorC:    make(chan error),
		StartC:    make(chan int),
		Count:     math.MaxInt32,
	}

	s.store.Fetch(req)

	var fetchedMessage *store.FetchedMessage

	opened := true
	for opened {
		select {
		case count := <-req.StartC:
			logger.WithField("count", count).
				Debug("Receiving messages for sync request from store")
		case fetchedMessage, opened = <-req.MessageC:
			if !opened {
				s.logger.WithField("requestNodeID", nodeID).
					Debug("Receive channel closed by the store for the sync request")
				return
			}

			// send message to node
			cmsg, err := s.cluster.newEncoderMessage(mtSyncMessage, &syncMessage{
				Partition: smr.Partition,
				ID:        fetchedMessage.ID,
				Message:   fetchedMessage.Message,
			})
			if err != nil {
				logger.WithError(err).
					WithField("fetchedMessage", fetchedMessage).
					Error("Error creating cluster message for fetched message")
				continue
			}

			err = s.cluster.sendMessageToNodeID(nodeID, cmsg)
			if err != nil {
				logger.WithError(err).
					WithField("clusterMesssage", cmsg).
					Error("Error sending sync message to node")
				continue
			}
		case <-s.stopC:
			s.logger.WithField("requestNodeID", nodeID).Debug("Stopping synchronization request loop")
			break
		}
	}
}

// syncMessage received data from another node after we made a request for a set
// of messages  it will decode the data into a *syncMessage and send it into the
// appropriate syncPartition processC channel
func (s *synchronizer) syncMessage(nodeID uint8, data []byte) error {
	if !s.inSyncID(nodeID) {
		return ErrNodeNotInSync
	}

	sm := &syncMessage{}
	err := sm.decode(data)
	if err != nil {
		logger.WithError(err).WithFields(log.Fields{
			"nodeID": nodeID,
			"data":   string(data),
		}).Error("Error decoding sync message received")
		return err
	}

	s.RLock()
	defer s.RUnlock()

	syncPartition, exists := s.syncPartitions[sm.Partition]
	if !exists {
		return ErrMissingSyncPartition
	}

	s.logger.WithFields(log.Fields{
		"sm":           sm,
		"syncPartiton": syncPartition,
	}).Debug("Processing received message")

	syncPartition.processC <- sm
	return nil
}

// keep state of fetching for a partition
type syncPartition struct {
	sync.RWMutex
	synchronizer *synchronizer

	localPartition  store.MessagePartition
	localStartMaxID uint64              // max message ID in the local store before the sync request
	nodes           map[uint8]partition // store nodes that have this partition and the info in does nodes
	lastID          uint64              // last fetched message ID

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
	// send request for the missing messages
	// get node with the highest message
	maxID, nodeID := sp.maxIDNode()

	partitionName := sp.localPartition.Name()
	cmsg, err := sp.synchronizer.cluster.newEncoderMessage(mtSyncMessageRequest, &syncMessageRequest{
		Partition: partitionName,
		StartID:   sp.lastID,
		EndID:     maxID,
	})
	if err != nil {
		sp.synchronizer.logger.WithError(err).Error("Error creating sync message request")
		return
	}

	err = sp.synchronizer.cluster.sendMessageToNodeID(nodeID, cmsg)
	if err != nil {
		sp.synchronizer.logger.WithError(err).WithFields(log.Fields{
			"nodeID":  nodeID,
			"StartID": sp.lastID,
			"EndID":   maxID,
		}).Error("Error sending sync message request to node")
		return
	}

	for {
		// wait to receive the message
		// end the loop in case we are stopping the process or we finished synchronizing
		select {
		case sm := <-sp.processC:
			err := sp.synchronizer.store.Store(partitionName, sm.ID, sm.Message)
			if err != nil {
				sp.synchronizer.logger.WithError(err).
					WithField("messageID", sm.ID).
					Error("Error storing synchronize message")
			}

			// end loop if we reached the end
			sp.lastID = sm.ID
			if sm.ID >= maxID {
				return
			}
		case <-sp.synchronizer.stopC:
			return
		}
	}
}

func (sp *syncPartition) maxIDNode() (max uint64, nodeID uint8) {
	for nid, p := range sp.nodes {
		if p.MaxID > max {
			max = p.MaxID
			nodeID = nid
		}
	}
	return
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
		logger.WithError(err).Error("Error retrieving store localPartitions")
		return nil
	}

	localPartitions := make(partitions, 0, len(messagePartitions))
	for _, p := range messagePartitions {
		localPartitions = append(localPartitions, partition{
			Name:  p.Name(),
			MaxID: p.MaxMessageID(),
		})
	}
	return &localPartitions
}

// send this struct to a node to request the messages between StartID and EndID
type syncMessageRequest struct {
	Partition string
	StartID   uint64
	EndID     uint64
}

func (smr *syncMessageRequest) encode() ([]byte, error) {
	return encode(smr)
}

func (smr *syncMessageRequest) decode(data []byte) error {
	return decode(smr, data)
}

type syncMessage struct {
	// Hold updated partition info
	Partition string
	ID        uint64
	Message   []byte
}

func (sm *syncMessage) encode() ([]byte, error) {
	return encode(sm)
}

func (sm *syncMessage) decode(data []byte) error {
	return decode(sm, data)
}
