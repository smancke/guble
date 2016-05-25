package store

import (
	"github.com/smancke/guble/protocol"

	"os"
	"path"
	"sync"
)

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

func (fms *FileMessageStore) MaxMessageId(partition string) (uint64, error) {
	p, err := fms.partitionStore(partition)
	if err != nil {
		return 0, err
	}
	return p.MaxMessageId()
}

func (fms *FileMessageStore) Stop() error {
	fms.mutex.Lock()
	defer fms.mutex.Unlock()

	var returnError error
	for key, partition := range fms.partitions {
		if err := partition.Close(); err != nil {
			returnError = err
			protocol.Err("error on closing message store partition %q: %v", key, err)
		}
		delete(fms.partitions, key)
	}
	return returnError
}

func (fms *FileMessageStore) StoreTx(partition string,
	callback func(msgId uint64) (msg []byte)) error {

	p, err := fms.partitionStore(partition)
	if err != nil {
		return err
	}
	return p.StoreTx(partition, callback)
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
		req.ErrorCallback <- err
		return
	}
	p.Fetch(req)
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
					return nil, err
				}
			} else {
				return nil, err
			}
		}
		var err error
		partitionStore, err = NewMessagePartition(dir, partition)
		if err != nil {
			return nil, err
		}
		fms.partitions[partition] = partitionStore
	}
	return partitionStore, nil
}
