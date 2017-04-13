package filestore

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cosminrentea/gobbler/server/store"

	"io"

	log "github.com/Sirupsen/logrus"
)

var (
	magicNumber       = []byte{42, 249, 180, 108, 82, 75, 222, 182}
	fileFormatVersion = []byte{1}
	messagesPerFile   = uint64(10000)
	indexEntrySize    = 20
)

const (
	gubleNodeIdBits    = 3
	sequenceBits       = 12
	gubleNodeIdShift   = sequenceBits
	timestampLeftShift = sequenceBits + gubleNodeIdBits
	gubleEpoch         = 1467714505012
)

type index struct {
	id     uint64
	offset uint64
	size   uint32
	fileID int
}

type messagePartition struct {
	basedir               string
	name                  string
	appendFile            *os.File
	indexFile             *os.File
	appendFilePosition    uint64
	maxMessageID          uint64
	sequenceNumber        uint64
	totalNumberOfMessages uint64
	entriesCount          uint64
	list                  *indexList
	fileCache             *cache

	sync.RWMutex
}

func newMessagePartition(basedir string, storeName string) (*messagePartition, error) {
	p := &messagePartition{
		basedir:   basedir,
		name:      storeName,
		list:      newIndexList(int(messagesPerFile)),
		fileCache: newCache(),
	}
	return p, p.initialize()
}

func (p *messagePartition) Name() string {
	return p.name
}

func (p *messagePartition) MaxMessageID() uint64 {
	p.RLock()
	defer p.RUnlock()

	return p.maxMessageID
}

func (p *messagePartition) Count() uint64 {
	p.RLock()
	defer p.RUnlock()

	return p.totalNumberOfMessages
}

func (p *messagePartition) initialize() error {
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
func (p *messagePartition) readIdxFiles() error {
	allFiles, err := ioutil.ReadDir(p.basedir)
	if err != nil {
		return err
	}

	var indexFilenames []string
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
		//add to total number of messages per partition
		p.totalNumberOfMessages += messagesPerFile

		// put entry in file cache
		p.fileCache.add(cEntry)
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
	//add the last part
	p.totalNumberOfMessages += uint64(p.list.len())
	back := p.list.back()

	if back != nil && back.id >= p.maxMessageID {
		p.maxMessageID = back.id
	}

	return nil
}

func (p *messagePartition) closeAppendFiles() error {
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
	if err != nil {
		return
	}
	defer file.Close()

	min, _, _, err := readIndexEntry(file, 0)
	if err != nil {
		return
	}
	max, _, _, err := readIndexEntry(file, int64((entriesInIndex-1)*uint64(indexEntrySize)))
	if err != nil {
		return
	}

	entry = &cacheEntry{min, max}
	return
}

func (p *messagePartition) createNextAppendFiles() error {
	filename := p.composeMsgFilenameForPosition(uint64(p.fileCache.length()))
	logger.WithField("filename", filename).Info("Creating next append files")

	appendfile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// write file header on new files
	if stat, _ := appendfile.Stat(); stat.Size() == 0 {
		p.appendFilePosition = uint64(stat.Size())

		_, err = appendfile.Write(magicNumber)
		if err != nil {
			return err
		}

		_, err = appendfile.Write(fileFormatVersion)
		if err != nil {
			return err
		}
	}

	indexfile, errIndex := os.OpenFile(p.composeIdxFilenameForPosition(uint64(p.fileCache.length())), os.O_RDWR|os.O_CREATE, 0666)
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

func (p *messagePartition) generateNextMsgID(nodeID uint8) (uint64, int64, error) {
	p.Lock()
	defer p.Unlock()

	//Get the local Timestamp
	currTime := time.Now()
	// timestamp in Seconds will be return to client
	timestamp := currTime.Unix()

	//Use the unixNanoTimestamp for generating id
	nanoTimestamp := currTime.UnixNano()

	if nanoTimestamp < gubleEpoch {
		err := fmt.Errorf("Clock is moving backwards. Rejecting requests until %d.", timestamp)
		return 0, 0, err
	}

	id := (uint64(nanoTimestamp-gubleEpoch) << timestampLeftShift) |
		(uint64(nodeID) << gubleNodeIdShift) | p.sequenceNumber

	p.sequenceNumber++

	logger.WithFields(log.Fields{
		"id":                  id,
		"messagePartition":    p.basedir,
		"localSequenceNumber": p.sequenceNumber,
		"currentNode":         nodeID,
	}).Debug("Generated id")

	return id, timestamp, nil
}

func (p *messagePartition) Close() error {
	p.Lock()
	defer p.Unlock()

	return p.closeAppendFiles()
}

func (p *messagePartition) DoInTx(fnToExecute func(maxMessageId uint64) error) error {
	p.Lock()
	defer p.Unlock()
	return fnToExecute(p.maxMessageID)
}

func (p *messagePartition) Store(msgID uint64, msg []byte) error {
	p.Lock()
	defer p.Unlock()

	return p.store(msgID, msg)
}

func (p *messagePartition) store(messageID uint64, data []byte) error {
	if p.entriesCount == messagesPerFile ||
		p.appendFile == nil ||
		p.indexFile == nil {

		logger.WithFields(log.Fields{
			"msgId":        messageID,
			"entriesCount": p.entriesCount,
			"fileCache":    p.fileCache,
		}).Debug("store")

		if err := p.closeAppendFiles(); err != nil {
			return err
		}

		if p.entriesCount == messagesPerFile {

			logger.WithFields(log.Fields{
				"msgId":        messageID,
				"entriesCount": p.entriesCount,
			}).Info("Dumping current file")

			//sort the indexFile
			err := p.rewriteSortedIdxFile(p.composeIdxFilenameForPosition(uint64(p.fileCache.length())))
			if err != nil {
				logger.WithError(err).Error("Error dumping file")
				return err
			}
			//Add items in the filecache
			p.fileCache.add(&cacheEntry{
				min: p.list.front().id,
				max: p.list.back().id,
			})

			//clear the current sorted cache
			p.list.clear()
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
	p.totalNumberOfMessages++

	logger.WithFields(log.Fields{
		"p.noOfEntriesInIndexFile": p.entriesCount,
		"msgID":                    messageID,
		"msgSize":                  uint32(len(data)),
		"msgOffset":                messageOffset,
		"filename":                 p.indexFile.Name(),
	}).Debug("Wrote in indexFile")

	//create entry for l
	e := &index{
		id:     messageID,
		offset: messageOffset,
		size:   uint32(len(data)),
		fileID: p.fileCache.length(),
	}
	p.list.insert(e)

	p.appendFilePosition += uint64(len(sizeAndID) + len(data))

	if messageID > p.maxMessageID {
		p.maxMessageID = messageID
	}

	return nil
}

// Fetch fetches a set of messages
func (p *messagePartition) Fetch(req *store.FetchRequest) {
	le := logger.WithFields(log.Fields{
		"partition": req.Partition,
		"startID":   req.StartID,
		"endID":     req.EndID,
		"Count":     req.Count,
	})
	le.Debug("Fetching")

	go func() {
		fetchList, err := p.calculateFetchList(req)

		if err != nil {
			log.WithField("err", err).Error("Error calculating list")
			req.ErrorC <- err
			return
		}
		req.StartC <- fetchList.len()

		err = p.fetchByFetchlist(fetchList, req)

		if err != nil {
			le.WithField("err", err).Error("Error calculating list")
			req.Error(err)
			return
		}
		req.Done()
	}()
}

// fetchByFetchlist fetches the messages in the supplied fetchlist and sends them to the message-channel
func (p *messagePartition) fetchByFetchlist(fetchList *indexList, req *store.FetchRequest) error {
	return fetchList.mapWithPredicate(func(index *index, _ int) error {
		if req.IsDone() {
			return store.ErrRequestDone
		}

		filename := p.composeMsgFilenameForPosition(uint64(index.fileID))
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

		req.Push(index.id, msg)
		return nil
	})
}

// calculateFetchList returns a list of fetchEntry records for all messages in the fetch request.
func (p *messagePartition) calculateFetchList(req *store.FetchRequest) (*indexList, error) {
	if req.Direction == 0 {
		req.Direction = 1
	}

	potentialEntries := newIndexList(0)

	// reading from IndexFiles
	// TODO: fix  prev when EndID logic will be done
	// prev specifies if we found anything in the previous list, in which case
	// it is possible the items to continue in the next list
	prev := false

	p.fileCache.RLock()

	for i, fce := range p.fileCache.entries {
		if fce.Contains(req) || (prev && potentialEntries.len() < req.Count) {
			prev = true

			l, err := p.loadIndexList(i)
			if err != nil {
				logger.WithError(err).Info("Error loading idx file in memory")
				return nil, err
			}

			potentialEntries.insert(l.extract(req).toSliceArray()...)
		} else {
			prev = false
		}
	}

	// Read from current cached value (the idx file which size is smaller than MESSAGE_PER_FILE
	if p.list.contains(req.StartID) || (prev && potentialEntries.len() < req.Count) {
		potentialEntries.insert(p.list.extract(req).toSliceArray()...)
	}

	// Currently potentialEntries contains a potentials IDs from any files and
	// from in memory. From this will select only Count.
	fetchList := potentialEntries.extract(req)

	p.fileCache.RUnlock()

	return fetchList, nil
}

func (p *messagePartition) rewriteSortedIdxFile(filename string) error {
	logger.WithFields(log.Fields{
		"filename": filename,
	}).Info("Dumping Sorted list")

	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	lastID := uint64(0)
	for i := 0; i < p.list.len(); i++ {
		item := p.list.get(i)

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
	offsetBuffer := make([]byte, indexEntrySize)
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
func writeIndexEntry(w io.WriterAt, id uint64, offset uint64, size uint32, pos uint64) error {
	position := int64(uint64(indexEntrySize) * pos)
	offsetBuffer := make([]byte, indexEntrySize)

	binary.LittleEndian.PutUint64(offsetBuffer, id)
	binary.LittleEndian.PutUint64(offsetBuffer[8:], offset)
	binary.LittleEndian.PutUint32(offsetBuffer[16:], size)

	if _, err := w.WriteAt(offsetBuffer, position); err != nil {
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
	entriesInIndex := uint64(stat.Size() / int64(indexEntrySize))
	return entriesInIndex, nil
}

// loadLastIndexFile will construct the current Sorted List for fetch entries which corresponds to the idx file with the biggest name
func (p *messagePartition) loadLastIndexList(filename string) error {
	logger.WithField("filename", filename).Info("Loading last index file")

	l, err := p.loadIndexList(p.fileCache.length())
	if err != nil {
		logger.WithError(err).Error("Error loading last index filename")
		return err
	}

	p.list = l
	p.entriesCount = uint64(l.len())

	return nil
}

// loadIndexFile will read a file and will return a sorted list for fetchEntries
func (p *messagePartition) loadIndexList(fileID int) (*indexList, error) {
	filename := p.composeIdxFilenameForPosition(uint64(fileID))
	l := newIndexList(int(messagesPerFile))
	logger.WithField("filename", filename).Debug("loadIndexFile")

	entriesInIndex, err := calculateNoEntries(filename)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(filename)
	if err != nil {
		logger.WithField("err", err).Error("os.Open failed")
		return nil, err
	}
	defer file.Close()

	for i := uint64(0); i < entriesInIndex; i++ {
		id, offset, size, err := readIndexEntry(file, int64(i*uint64(indexEntrySize)))
		logger.WithFields(log.Fields{
			"offset": offset,
			"size":   size,
			"id":     id,
			"err":    err,
		}).Debug("readIndexEntry")

		if err != nil {
			logger.WithField("err", err).Error("Read error")
			return nil, err
		}

		e := &index{
			id:     id,
			size:   size,
			offset: offset,
			fileID: fileID,
		}
		l.insert(e)
		logger.WithField("len", l.len()).Debug("loadIndexFile")
	}
	return l, nil
}

func (p *messagePartition) composeMsgFilenameForPosition(value uint64) string {
	return filepath.Join(p.basedir, fmt.Sprintf("%s-%020d.msg", p.name, value))
}

func (p *messagePartition) composeIdxFilenameForPosition(value uint64) string {
	return filepath.Join(p.basedir, fmt.Sprintf("%s-%020d.idx", p.name, value))
}
