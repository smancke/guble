package store

import (
	"errors"
	"os"
	"path"
	"sync"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/protocol"
)

var messageStoreLogger = log.WithFields(log.Fields{
	"module": "messageStore",
})

// FileMessageStore is an implementation of the MessageStore interface based on files
type FileMessageStore struct {
	partitions map[string]*MessagePartition
	basedir    string
	mutex      sync.RWMutex
}

func NewFileMessageStore(basedir string) *FileMessageStore {
	return &FileMessageStore{
		partitions: make(map[string]*MessagePartition),
		basedir:    basedir,
	}
}

func (fms *FileMessageStore) MaxMessageID(partition string) (uint64, error) {
	p, err := fms.partitionStore(partition)
	if err != nil {
		return 0, err
	}
	return p.MaxMessageId()
}

func (fms *FileMessageStore) Stop() error {
	fms.mutex.Lock()
	defer fms.mutex.Unlock()
	messageStoreLogger.Info("Stop")
	var returnError error
	for key, partition := range fms.partitions {
		if err := partition.Close(); err != nil {
			returnError = err
			messageStoreLogger.WithFields(log.Fields{
				"key": key,
				"err": err,
			}).Error("Error on closing message store partition for")
		}
		delete(fms.partitions, key)
	}
	return returnError
}

func (fms *FileMessageStore) GenerateNextMsgId(msgPathPartition string, nodeID int) (uint64, int64, error) {
	p, err := fms.partitionStore(msgPathPartition)
	if err != nil {
		return 0, 0, err
	}
	return p.generateNextMsgId(nodeID)
}

func (fms *FileMessageStore) StoreMessage(message *protocol.Message, nodeID int) (int, error) {
	partitionName := message.Path.Partition()

	// If nodeID is zero means we are running in standalone more, otherwise
	// if the message has no nodeID it means it was received by this node
	if nodeID == 0 || message.NodeID == 0 {
		id, ts, err := fms.GenerateNextMsgId(partitionName, nodeID)

		if err != nil {
			messageStoreLogger.WithError(err).Error("Generation of id failed")
			return 0, err
		}

		message.ID = id
		message.Time = ts
		message.NodeID = nodeID

		log.WithFields(log.Fields{
			"generatedID":   id,
			"generatedTime": message.Time,
		}).Info("Locally generated ID for message")
	}

	data := message.Bytes()

	if err := fms.Store(partitionName, message.ID, message.Bytes()); err != nil {
		messageStoreLogger.
			WithError(err).WithField("partition", partitionName).
			Error("Error storing locally generated  messagein partition")
		return 0, err
	}

	messageStoreLogger.WithFields(log.Fields{
		"id":            message.ID,
		"ts":            message.Time,
		"partition":     partitionName,
		"messageUserID": message.UserID,
		"nodeID":        nodeID,
	}).Info("Stored message")

	return len(data), nil
}

// Store stores a message within a partition
func (fms *FileMessageStore) Store(partition string, msgId uint64, msg []byte) error {
	p, err := fms.partitionStore(partition)
	if err != nil {
		return err
	}
	return p.Store(msgId, msg)
}

// Fetch asynchronously fetches a set of messages defined by the fetch request
func (fms *FileMessageStore) Fetch(req FetchRequest) {
	p, err := fms.partitionStore(req.Partition)
	if err != nil {
		req.ErrorC <- err
		return
	}
	p.Fetch(&req)
}

func (fms *FileMessageStore) DoInTx(partition string, fnToExecute func(maxMessageId uint64) error) error {
	p, err := fms.partitionStore(partition)
	if err != nil {
		return err
	}
	return p.DoInTx(fnToExecute)
}

func (fms *FileMessageStore) partitionStore(partition string) (*MessagePartition, error) {
	fms.mutex.Lock()
	defer fms.mutex.Unlock()

	partitionStore, exist := fms.partitions[partition]
	if !exist {
		dir := path.Join(fms.basedir, partition)
		if _, err := os.Stat(dir); err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(dir, 0700); err != nil {
					messageStoreLogger.WithField("err", err).Error("partitionStore")
					return nil, err
				}
			} else {
				messageStoreLogger.WithField("err", err).Error("partitionStore")
				return nil, err
			}
		}
		var err error
		partitionStore, err = NewMessagePartition(dir, partition)
		if err != nil {
			messageStoreLogger.WithField("err", err).Error("partitionStore")
			return nil, err
		}
		fms.partitions[partition] = partitionStore
	}
	return partitionStore, nil
}

func (fms *FileMessageStore) Check() error {
	var stat syscall.Statfs_t
	wd, err := os.Getwd()
	if err != nil {
		messageStoreLogger.WithField("err", err).Error("Check() failed")
		return err
	}
	syscall.Statfs(wd, &stat)

	// available space in bytes = available blocks * size per block
	freeSpace := stat.Bavail * uint64(stat.Bsize)
	// total space in bytes = total system blocks * size per block
	totalSpace := stat.Blocks * uint64(stat.Bsize)

	usedSpacePercentage := 1 - (float64(freeSpace) / float64(totalSpace))

	if usedSpacePercentage > 0.95 {
		errorMessage := "Disk is almost full."
		messageStoreLogger.WithField("usedDiskSpacePercentage", usedSpacePercentage).Warn(errorMessage)
		return errors.New(errorMessage)
	}

	return nil
}
