package file

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/smancke/guble/store"

	log "github.com/Sirupsen/logrus"
)

var (
	MAGIC_NUMBER        = []byte{42, 249, 180, 108, 82, 75, 222, 182}
	FILE_FORMAT_VERSION = []byte{1}
	MESSAGES_PER_FILE   = uint64(10000)
	INDEX_ENTRY_SIZE    = 20
)

const (
	GubleNodeIdBits    = 3
	SequenceBits       = 12
	GubleNodeIdShift   = SequenceBits
	TimestampLeftShift = SequenceBits + GubleNodeIdBits
	GubleEpoch         = 1467714505012
)

type Index struct {
	id     uint64
	offset uint64
	size   uint32
	fileID int
}

type MessagePartition struct {
	basedir            string
	name               string
	appendFile         *os.File
	indexFile          *os.File
	appendFilePosition uint64
	maxMessageID       uint64
	sequenceNumber     uint64

	entriesCount uint64 //TODO  MAYBE USE ONLY ONE  FROM THE noOfEntriesInIndexFile AND localSequenceNumber
	list         *IndexList
	fileCache    *cache

	sync.RWMutex
}

func NewMessagePartition(basedir string, storeName string) (*MessagePartition, error) {
	p := &MessagePartition{
		basedir:   basedir,
		name:      storeName,
		list:      newList(int(MESSAGES_PER_FILE)),
		fileCache: newCache(),
	}
	return p, p.initialize()
}

func (p *MessagePartition) initialize() error {
	p.Lock()
	defer p.Unlock()

	// reset the cache entries
	p.fileCache = newCache()
	err := p.readIdxFiles()
	if err != nil {
		logger.WithField("err", err).Error("MessagePartition error on scanFiles")
		return err
	}

	return nil
}

// Returns the start messages ids for all available message files
// in a sorted list
func (p *MessagePartition) readIdxFiles() error {
	allFiles, err := ioutil.ReadDir(p.basedir)
	if err != nil {
		return err
	}

	indexFilenames := make([]string, 0)
	for _, fileInfo := range allFiles {
		if strings.HasPrefix(fileInfo.Name(), p.name+"-") && strings.HasSuffix(fileInfo.Name(), ".idx") {
			fileIDString := filepath.Join(p.basedir, fileInfo.Name())
			logger.WithField("name", fileIDString).Info("Index name")
			indexFilenames = append(indexFilenames, fileIDString)
		}
	}

	// if no .idx file are found.. there is nothing to load
	if len(indexFilenames) == 0 {
		logger.Info("No .idx files found")
		return nil
	}

	//load the filecache from all the files
	logger.WithFields(log.Fields{
		"filenames":  indexFilenames,
		"totalFiles": len(indexFilenames),
	}).Info("Found files")

	for i := 0; i < len(indexFilenames)-1; i++ {
		cEntry, err := readCacheEntryFromIdxFile(indexFilenames[i])
		if err != nil {
			logger.WithFields(log.Fields{
				"idxFilename": indexFilenames[i],
				"err":         err,
			}).Error("Error loading existing .idxFile")
			return err
		}

		// put entry in file cache
		p.fileCache.Append(cEntry)
		logger.
			WithField("entries", p.fileCache.entries).
			WithField("filename", indexFilenames[i]).
			Error("Entries")

		// check the message id's for max value
		if cEntry.max >= p.maxMessageID {
			p.maxMessageID = cEntry.max
		}
	}

	// read the  idx file with   biggest id and load in the sorted cache
	if err := p.loadLastIndexList(indexFilenames[len(indexFilenames)-1]); err != nil {
		logger.WithFields(log.Fields{
			"idxFilename": indexFilenames[(len(indexFilenames) - 1)],
			"err":         err,
		}).Error("Error loading last .idx file")
		return err
	}

	if p.list.Back().id >= p.maxMessageID {
		p.maxMessageID = p.list.Back().id
	}

	return nil
}

func (p *MessagePartition) MaxMessageID() (uint64, error) {
	p.Lock()
	defer p.Unlock()

	return p.maxMessageID, nil
}

func (p *MessagePartition) closeAppendFiles() error {
	if p.appendFile != nil {
		if err := p.appendFile.Close(); err != nil {
			if p.indexFile != nil {
				defer p.indexFile.Close()
			}
			return err
		}
		p.appendFile = nil
	}

	if p.indexFile != nil {
		err := p.indexFile.Close()
		p.indexFile = nil
		return err
	}
	return nil
}

// readCacheEntryFromIdxFile  reads the first and last entry from a idx file which should be sorted
func readCacheEntryFromIdxFile(filename string) (entry *cacheEntry, err error) {
	entriesInIndex, err := calculateNoEntries(filename)
	if err != nil {
		return
	}

	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		return
	}

	min, _, _, err := readIndexEntry(file, 0)
	if err != nil {
		return
	}
	max, _, _, err := readIndexEntry(file, int64((entriesInIndex-1)*uint64(INDEX_ENTRY_SIZE)))
	if err != nil {
		return
	}

	entry = &cacheEntry{min, max}
	return
}

func (p *MessagePartition) createNextAppendFiles() error {
	filename := p.composeMsgFilename()
	logger.WithField("filename", filename).Info("Creating next append files")

	appendfile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// write file header on new files
	if stat, _ := appendfile.Stat(); stat.Size() == 0 {
		p.appendFilePosition = uint64(stat.Size())

		_, err = appendfile.Write(MAGIC_NUMBER)
		if err != nil {
			return err
		}

		_, err = appendfile.Write(FILE_FORMAT_VERSION)
		if err != nil {
			return err
		}
	}

	indexfile, errIndex := os.OpenFile(p.composeIdxFilename(), os.O_RDWR|os.O_CREATE, 0666)
	if errIndex != nil {
		defer appendfile.Close()
		defer os.Remove(appendfile.Name())
		return err
	}

	p.appendFile = appendfile
	p.indexFile = indexfile
	stat, err := appendfile.Stat()
	if err != nil {
		return err
	}
	p.appendFilePosition = uint64(stat.Size())

	return nil
}

func (p *MessagePartition) generateNextMsgID(nodeID int) (uint64, int64, error) {
	p.Lock()
	defer p.Unlock()

	//Get the local Timestamp
	currTime := time.Now()
	// timestamp in Seconds will be return to client
	timestamp := currTime.Unix()

	//Use the unixNanoTimestamp for generating id
	nanoTimestamp := currTime.UnixNano()

	if nanoTimestamp < GubleEpoch {
		err := fmt.Errorf("Clock is moving backwards. Rejecting requests until %d.", timestamp)
		return 0, 0, err
	}

	id := (uint64(nanoTimestamp-GubleEpoch) << TimestampLeftShift) |
		(uint64(nodeID) << GubleNodeIdShift) | p.sequenceNumber

	p.sequenceNumber++

	logger.WithFields(log.Fields{
		"id":                  id,
		"messagePartition":    p.basedir,
		"localSequenceNumber": p.sequenceNumber,
		"currentNode":         nodeID,
	}).Info("Generated id")

	return id, timestamp, nil
}

func (p *MessagePartition) Close() error {
	p.Lock()
	defer p.Unlock()

	return p.closeAppendFiles()
}

func (p *MessagePartition) DoInTx(fnToExecute func(maxMessageId uint64) error) error {
	p.Lock()
	defer p.Unlock()
	return fnToExecute(p.maxMessageID)
}

func (p *MessagePartition) Store(msgId uint64, msg []byte) error {
	p.Lock()
	defer p.Unlock()

	return p.store(msgId, msg)
}

func (p *MessagePartition) store(messageID uint64, data []byte) error {
	if p.entriesCount == MESSAGES_PER_FILE ||
		p.appendFile == nil ||
		p.indexFile == nil {

		logger.WithFields(log.Fields{
			"msgId":        messageID,
			"entriesCount": p.entriesCount,
			"fileCache":    p.fileCache,
		}).Debug("In store")

		if err := p.closeAppendFiles(); err != nil {
			return err
		}

		if p.entriesCount == MESSAGES_PER_FILE {

			logger.WithFields(log.Fields{
				"msgId":        messageID,
				"entriesCount": p.entriesCount,
			}).Info("Dumping current file ")

			//sort the indexFile
			err := p.rewriteSortedIdxFile(p.composeIdxFilename())
			if err != nil {
				logger.WithField("err", err).Error("Error dumping file")
				return err
			}
			//Add items in the filecache
			p.fileCache.Append(&cacheEntry{
				min: p.list.Front().id,
				max: p.list.Back().id,
			})

			//clear the current sorted cache
			p.list.Clear()
			p.entriesCount = 0
		}

		if err := p.createNextAppendFiles(); err != nil {
			return err
		}
	}

	// write the message size and the message id: 32 bit and 64 bit, so 12 bytes
	sizeAndID := make([]byte, 12)
	binary.LittleEndian.PutUint32(sizeAndID, uint32(len(data)))
	binary.LittleEndian.PutUint64(sizeAndID[4:], messageID)

	if _, err := p.appendFile.Write(sizeAndID); err != nil {
		return err
	}

	// write the message
	if _, err := p.appendFile.Write(data); err != nil {
		return err
	}

	// write the index entry to the index file
	messageOffset := p.appendFilePosition + uint64(len(sizeAndID))
	err := writeIndexEntry(p.indexFile, messageID, messageOffset, uint32(len(data)), p.entriesCount)
	if err != nil {
		return err
	}
	p.entriesCount++

	log.WithFields(log.Fields{
		"p.noOfEntriesInIndexFile": p.entriesCount,
		"msgID":                    messageID,
		"msgSize":                  uint32(len(data)),
		"msgOffset":                messageOffset,
		"filename":                 p.indexFile.Name(),
	}).Debug("Wrote in indexFile")

	//create entry for l
	e := &Index{
		id:     messageID,
		offset: messageOffset,
		size:   uint32(len(data)),
		fileID: p.fileCache.Len(),
	}
	p.list.Insert(e)

	p.appendFilePosition += uint64(len(sizeAndID) + len(data))

	if messageID >= messageID {
		p.maxMessageID = messageID
	}

	return nil
}

// Fetch fetches a set of messages
func (p *MessagePartition) Fetch(req *store.FetchRequest) {
	log.WithField("fetchRequest", *req).Debug("Fetching")

	go func() {
		fetchList, err := p.calculateFetchList(req)

		if err != nil {
			log.WithField("err", err).Error("Error calculating list")
			req.ErrorC <- err
			return
		}

		req.StartC <- fetchList.Len()

		err = p.fetchByFetchlist(fetchList, req.MessageC)

		if err != nil {
			log.WithField("err", err).Error("Error calculating list")
			req.ErrorC <- err
			return
		}
		close(req.MessageC)
	}()
}

// fetchByFetchlist fetches the messages in the supplied fetchlist and sends them to the message-channel
func (p *MessagePartition) fetchByFetchlist(fetchList *IndexList, messageC chan store.FetchedMessage) error {
	return fetchList.Do(func(index *Index, _ int) error {
		filename := p.composeMsgFilenameForID(uint64(index.fileID))
		file, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer file.Close()

		msg := make([]byte, index.size, index.size)
		_, err = file.ReadAt(msg, int64(index.offset))
		if err != nil {
			logger.WithFields(log.Fields{
				"err":    err,
				"offset": index.offset,
			}).Error("Error ReadAt")
			return err
		}

		messageC <- store.FetchedMessage{index.id, msg}
		return nil
	})
}

// calculateFetchList returns a list of fetchEntry records for all messages in the fetch request.
func (p *MessagePartition) calculateFetchList(req *store.FetchRequest) (*IndexList, error) {
	if req.Direction == 0 {
		req.Direction = 1
	}

	potentialEntries := newList(0)

	// reading from IndexFiles
	// TODO: fix  prev when EndID logic will be done
	// prev specifies if we found anything in the previous list, in which case
	// it is possible the items to continue in the next list
	prev := false

	p.fileCache.RLock()

	for i, fce := range p.fileCache.entries {
		if fce.Contains(req) || (prev && potentialEntries.Len() < req.Count) {
			prev = true

			l, err := p.loadIndexList(i)
			if err != nil {
				logger.WithError(err).Info("Error loading idx file in memory")
				return nil, err
			}

			potentialEntries.Insert(l.Extract(req).Items()...)
		} else {
			prev = false
		}
	}

	// Read from current cached value (the idx file which size is smaller than MESSAGE_PER_FILE
	if p.list.Contains(req.StartID) || (prev && potentialEntries.Len() < req.Count) {
		potentialEntries.Insert(p.list.Extract(req).Items()...)
	}

	// Currently potentialEntries contains a potentials msgIDs from any files and from inMemory.From this will select only Count Id.
	fetchList := potentialEntries.Extract(req)

	p.fileCache.RUnlock()

	return fetchList, nil
}

func (p *MessagePartition) rewriteSortedIdxFile(filename string) error {
	logger.WithFields(log.Fields{
		"filename": filename,
	}).Info("Dumping Sorted list")

	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0666)
	defer file.Close()
	if err != nil {
		return err
	}

	lastID := uint64(0)
	for i := 0; i < p.list.Len(); i++ {
		item := p.list.Get(i)

		if lastID >= item.id {
			logger.WithFields(log.Fields{
				"err":      err,
				"filename": filename,
			}).Error("Sorted list is not sorted")

			return err
		}
		lastID = item.id

		err := writeIndexEntry(file, item.id, item.offset, item.size, uint64(i))
		logger.WithFields(log.Fields{
			"curMsgId": item.id,
			"err":      err,
			"pos":      i,
			"filename": file.Name(),
		}).Debug("Wrote while dumpSortedIndexFile")

		if err != nil {
			logger.WithField("err", err).Error("Error writing indexfile in sorted way.")
			return err
		}
	}
	return nil

}

// readIndexEntry reads from a .idx file from the given `position` the msgID msgOffset and msgSize
func readIndexEntry(file *os.File, position int64) (uint64, uint64, uint32, error) {
	offsetBuffer := make([]byte, INDEX_ENTRY_SIZE)
	if _, err := file.ReadAt(offsetBuffer, position); err != nil {
		logger.WithFields(log.Fields{
			"err":      err,
			"file":     file.Name(),
			"indexPos": position,
		}).Error("Error reading index entry")
		return 0, 0, 0, err
	}

	id := binary.LittleEndian.Uint64(offsetBuffer)
	offset := binary.LittleEndian.Uint64(offsetBuffer[8:])
	size := binary.LittleEndian.Uint32(offsetBuffer[16:])
	return id, offset, size, nil
}

// writeIndexEntry write in a .idx file to  the given `pos` the msgIDm msgOffset and msgSize
func writeIndexEntry(file *os.File, id uint64, offset uint64, size uint32, pos uint64) error {
	position := int64(uint64(INDEX_ENTRY_SIZE) * pos)
	offsetBuffer := make([]byte, INDEX_ENTRY_SIZE)

	binary.LittleEndian.PutUint64(offsetBuffer, id)
	binary.LittleEndian.PutUint64(offsetBuffer[8:], offset)
	binary.LittleEndian.PutUint32(offsetBuffer[16:], size)

	if _, err := file.WriteAt(offsetBuffer, position); err != nil {
		logger.WithFields(log.Fields{
			"err":      err,
			"position": position,
			"id":       id,
		}).Error("Error writing index entry")
		return err
	}
	return nil
}

// calculateNoEntries reads the idx file with name `filename` and will calculate how many entries are
func calculateNoEntries(filename string) (uint64, error) {
	stat, err := os.Stat(filename)
	if err != nil {
		logger.WithField("err", err).Error("Stat failed")
		return 0, err
	}
	entriesInIndex := uint64(stat.Size() / int64(INDEX_ENTRY_SIZE))
	return entriesInIndex, nil
}

// loadLastIndexFile will construct the current Sorted List for fetch entries which corresponds to the idx file with the biggest name
func (p *MessagePartition) loadLastIndexList(filename string) error {
	logger.WithField("filename", filename).Info("Loading last index file")

	l, err := p.loadIndexList(p.fileCache.Len())
	if err != nil {
		logger.WithError(err).Error("Error loading filename")
		return err
	}

	p.list = l
	p.entriesCount = uint64(l.Len())

	return nil
}

// loadIndexFile will read a file and will return a sorted list for fetchEntries
func (p *MessagePartition) loadIndexList(fileID int) (*IndexList, error) {
	filename := p.composeIdxFilenameForID(uint64(fileID))
	l := newList(int(MESSAGES_PER_FILE))
	logger.WithField("filename", filename).Debug("loadIndexFile")

	entriesInIndex, err := calculateNoEntries(filename)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		logger.WithField("err", err).Error("os.Open failed")
		return nil, err
	}

	for i := uint64(0); i < entriesInIndex; i++ {
		id, offset, size, err := readIndexEntry(file, int64(i*uint64(INDEX_ENTRY_SIZE)))
		logger.WithFields(log.Fields{
			"offset": offset,
			"size":   size,
			"id":     id,
			"err":    err,
		}).Debug("readIndexEntry")

		if err != nil {
			log.WithField("err", err).Error("Read error")
			return nil, err
		}

		e := &Index{
			id:     id,
			size:   size,
			offset: offset,
			fileID: fileID,
		}
		l.Insert(e)
		logger.WithField("lenl", l.Len()).Debug("loadIndexFile")
	}
	return l, nil
}

func (p *MessagePartition) composeMsgFilename() string {
	return filepath.Join(p.basedir, fmt.Sprintf("%s-%020d.msg", p.name, uint64(p.fileCache.Len())))
}

func (p *MessagePartition) composeMsgFilenameForID(value uint64) string {
	return filepath.Join(p.basedir, fmt.Sprintf("%s-%020d.msg", p.name, value))
}

func (p *MessagePartition) composeIdxFilename() string {
	return filepath.Join(p.basedir, fmt.Sprintf("%s-%020d.idx", p.name, uint64(p.fileCache.Len())))
}

func (p *MessagePartition) composeIdxFilenameForID(value uint64) string {
	return filepath.Join(p.basedir, fmt.Sprintf("%s-%020d.idx", p.name, value))
}
