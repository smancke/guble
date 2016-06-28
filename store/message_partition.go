package store

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/smancke/guble/gubled/config"

	log "github.com/Sirupsen/logrus"
)

var (
	MAGIC_NUMBER        = []byte{42, 249, 180, 108, 82, 75, 222, 182}
	FILE_FORMAT_VERSION = []byte{1}
	MESSAGES_PER_FILE   = uint64(10000)
	INDEX_ENTRY_SIZE    = 12
)

const (
	defaultInitialCapacity = 128
	WorkerIdBits           = 5
	//DatacenterIdBits   = 5
	SequenceBits       = 12
	WorkerIdShift      = SequenceBits
	TimestampLeftShift = SequenceBits + WorkerIdBits //+ DatacenterIdBits
	GubleEpoch         = 1467024972
)

type fetchEntry struct {
	messageID uint64
	fileId    uint64
	offset    int64
	size      int
}

type MessagePartition struct {
	basedir                 string
	name                    string
	appendFile              *os.File
	indexFile               *os.File
	appendFirstID           uint64
	appendLastID            uint64
	appendFileWritePosition uint64
	maxMessageID            uint64
	currentIndex            uint64
	mutex                   *sync.RWMutex
}

func NewMessagePartition(basedir string, storeName string) (*MessagePartition, error) {
	p := &MessagePartition{
		basedir: basedir,
		name:    storeName,
		mutex:   &sync.RWMutex{},
	}
	return p, p.initialize()
}

func (p *MessagePartition) initialize() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	fileList, err := p.scanFiles()
	if err != nil {
		messageStoreLogger.WithField("err", err).Error("MessagePartition error on scanFiles")
		return err
	}
	if len(fileList) == 0 {
		p.maxMessageID = 0
	} else {
		var err error
		p.maxMessageID, err = p.calculateMaxMessageIDFromIndex(fileList[len(fileList)-1])
		if err != nil {
			messageStoreLogger.WithField("err", err).Error("MessagePartition error on calculateMaxMessageIdFromIndex")
			return err
		}
	}
	return nil
}

// calculateMaxMessageIdFromIndex returns the max message id for a message file
func (p *MessagePartition) calculateMaxMessageIDFromIndex(fileId uint64) (uint64, error) {
	stat, err := os.Stat(p.indexFilenameByMessageId(fileId))
	if err != nil {
		return 0, err
	}
	entriesInIndex := uint64(stat.Size() / int64(INDEX_ENTRY_SIZE))

	return entriesInIndex - 1 + fileId, nil
}

// Returns the start messages ids for all available message files
// in a sorted list
func (p *MessagePartition) scanFiles() ([]uint64, error) {
	result := []uint64{}
	allFiles, err := ioutil.ReadDir(p.basedir)
	if err != nil {
		return nil, err
	}
	for _, fileInfo := range allFiles {
		if strings.HasPrefix(fileInfo.Name(), p.name+"-") &&
			strings.HasSuffix(fileInfo.Name(), ".idx") {
			fileIdString := fileInfo.Name()[len(p.name)+1 : len(fileInfo.Name())-4]
			if fileId, err := strconv.ParseUint(fileIdString, 10, 64); err == nil {
				result = append(result, fileId)
			}
		}
	}
	return result, nil
}

func (p *MessagePartition) MaxMessageID() (uint64, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

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

func (p *MessagePartition) createNextAppendFiles(msgId uint64) error {
	firstMessageIdForFile := p.firstMessageIdForFile(msgId)

	appendfile, err := os.OpenFile(p.filenameByMessageId(firstMessageIdForFile), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// write file header on new files
	if stat, _ := appendfile.Stat(); stat.Size() == 0 {
		p.appendFileWritePosition = uint64(stat.Size())

		_, err = appendfile.Write(MAGIC_NUMBER)
		if err != nil {
			return err
		}

		_, err = appendfile.Write(FILE_FORMAT_VERSION)
		if err != nil {
			return err
		}
	}

	indexfile, errIndex := os.OpenFile(p.indexFilenameByMessageId(firstMessageIdForFile), os.O_RDWR|os.O_CREATE, 0666)
	if errIndex != nil {
		defer appendfile.Close()
		defer os.Remove(appendfile.Name())
		return err
	}

	p.appendFile = appendfile
	p.indexFile = indexfile
	p.appendFirstID = firstMessageIdForFile
	p.appendLastID = firstMessageIdForFile + MESSAGES_PER_FILE - 1
	stat, err := appendfile.Stat()
	if err != nil {
		return err
	}
	p.appendFileWritePosition = uint64(stat.Size())

	return nil
}

func (p *MessagePartition) generateNextMsgId(timestamp int64) (uint64, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if timestamp < GubleEpoch {
		err := fmt.Errorf("Clock is moving backwards. Rejecting requests until %d.", timestamp)
		return 0, err
	}

	id := (uint64(timestamp-GubleEpoch) << TimestampLeftShift) |
		(uint64(*config.Cluster.NodeID) << WorkerIdShift) | p.currentIndex

	p.currentIndex++

	messageStoreLogger.WithFields(log.Fields{
		"id":                  id,
		"messagePartition":    p.basedir,
		"localSequenceNumber": p.currentIndex,
	}).Info("+Generated id")

	return id, nil
}

func (p *MessagePartition) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.closeAppendFiles()
}

func (p *MessagePartition) DoInTx(fnToExecute func(maxMessageId uint64) error) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return fnToExecute(p.maxMessageID)
}

func (p *MessagePartition) Store(msgId uint64, msg []byte) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.store(msgId, msg)
}

func (p *MessagePartition) store(msgId uint64, msg []byte) error {
	// TODO MARIAN remove this after finishing the priority queue
	//if msgId != 1+p.maxMessageId {
	//	return fmt.Errorf("MessagePartition: Invalid message id for partition %v. Next id should be %v, but was %q",
	//		p.name, 1+p.maxMessageId, msgId)
	//}
	if msgId > p.appendLastID ||
		p.appendFile == nil ||
		p.indexFile == nil {

		if err := p.closeAppendFiles(); err != nil {
			return err
		}
		if err := p.createNextAppendFiles(msgId); err != nil {
			return err
		}
	}

	// write the message size and the message id: 32 bit and 64 bit, so 12 bytes
	sizeAndId := make([]byte, 12)
	binary.LittleEndian.PutUint32(sizeAndId, uint32(len(msg)))
	binary.LittleEndian.PutUint64(sizeAndId[4:], msgId)
	if _, err := p.appendFile.Write(sizeAndId); err != nil {
		return err
	}

	// write the message
	if _, err := p.appendFile.Write(msg); err != nil {
		return err
	}

	// write the index entry to the index file
	indexPosition := int64(uint64(INDEX_ENTRY_SIZE) * (msgId % MESSAGES_PER_FILE))
	messageOffset := p.appendFileWritePosition + uint64(len(sizeAndId))
	messageOffsetBuff := make([]byte, INDEX_ENTRY_SIZE)
	binary.LittleEndian.PutUint64(messageOffsetBuff, messageOffset)
	binary.LittleEndian.PutUint32(messageOffsetBuff[8:], uint32(len(msg)))

	if _, err := p.indexFile.WriteAt(messageOffsetBuff, indexPosition); err != nil {
		return err
	}

	p.appendFileWritePosition += uint64(len(sizeAndId) + len(msg))

	p.maxMessageID = msgId

	return nil
}

// Fetch fetches a set of messages
func (p *MessagePartition) Fetch(req FetchRequest) {
	go func() {
		fetchList, err := p.calculateFetchList(req)
		if err != nil {
			req.ErrorC <- err
			return
		}

		req.StartC <- len(fetchList)

		err = p.fetchByFetchlist(fetchList, req.MessageC)
		if err != nil {
			req.ErrorC <- err
			return
		}
		close(req.MessageC)
	}()
	//log.WithField("req", req.StartId).Error("End Fetch")
}

// fetchByFetchlist fetches the messages in the supplied fetchlist and sends them to the message-channel
func (p *MessagePartition) fetchByFetchlist(fetchList []fetchEntry, messageC chan MessageAndID) error {
	var fileId uint64
	var file *os.File
	var err error
	var lastMsgId uint64
	for _, f := range fetchList {
		if lastMsgId == 0 {
			lastMsgId = f.messageID - 1
		}
		lastMsgId = f.messageID

		log.WithFields(log.Fields{
			"lastMsgId":   lastMsgId,
			"f.messageId": f.messageID,
			"fileID":      f.fileId,
		}).Debug("fetchByFetchlist for ")

		// ensure, that we read from the correct file
		if file == nil || fileId != f.fileId {
			file, err = p.checkoutMessagefile(f.fileId)
			if err != nil {
				log.WithField("err", err).Error("Error checkoutMessagefile")
				return err
			}
			defer p.releaseMessagefile(f.fileId, file)
			fileId = f.fileId
		}

		msg := make([]byte, f.size, f.size)
		_, err = file.ReadAt(msg, f.offset)
		if err != nil {
			log.WithField("err", err).Error("Error ReadAt")
			return err
		}
		messageC <- MessageAndID{f.messageID, msg}
	}
	return nil
}

// calculateFetchList returns a list of fetchEntry records for all messages in the fetch request.
func (p *MessagePartition) calculateFetchList(req FetchRequest) ([]fetchEntry, error) {

	log.WithFields(log.Fields{
		"req": req,
	}).Debug("calculateFetchList ")
	if req.Direction == 0 {
		req.Direction = 1
	}
	nextId := req.StartID
	initialCap := req.Count
	if initialCap > defaultInitialCapacity {
		initialCap = defaultInitialCapacity
	}
	result := make([]fetchEntry, 0, initialCap)
	var file *os.File
	var fileId uint64

	log.WithFields(log.Fields{
		"nextId":     nextId,
		"initialCap": initialCap,
		"len_resutl": len(result),
		"reqCount":   req.Count,
	}).Debug("Fetch Before for")

	for len(result) < req.Count && nextId >= 0 {
		nextFileId := p.firstMessageIdForFile(nextId)

		log.WithFields(log.Fields{
			"nextId":     nextId,
			"nextFileId": nextFileId,
			"fileId":     fileId,
			"initialCap": initialCap,
		}).Debug("calculateFetchList FOR")

		// ensure, that we read from the correct file
		if file == nil || nextFileId != fileId {
			var err error
			file, err = p.checkoutIndexfile(nextFileId)
			if err != nil {
				if os.IsNotExist(err) {
					log.WithField("result", req).Error("IsNotExist")
					return result, nil
				}
				log.WithField("err", err).Error("checkoutIndexfile")
				return nil, err
			}
			defer p.releaseIndexfile(fileId, file)
			fileId = nextFileId
		}

		indexPosition := int64(uint64(INDEX_ENTRY_SIZE) * (nextId % MESSAGES_PER_FILE))

		msgOffset, msgSize, err := readIndexEntry(file, indexPosition)
		log.WithFields(log.Fields{
			"indexPosition": indexPosition,
			"msgOffset":     msgOffset,
			"msgSize":       msgSize,
			"err":           err,
		}).Debug("readIndexEntry")

		if err != nil {
			if err.Error() == "EOF" {
				return result, nil // we reached the end of the index
			}
			log.WithField("err", err).Error("EOF")
			return nil, err
		}

		if msgOffset != uint64(0) { // only append, if the message exists
			result = append(result, fetchEntry{
				messageID: nextId,
				fileId:    fileId,
				offset:    int64(msgOffset),
				size:      int(msgSize),
			})
		}

		nextId += uint64(req.Direction)
	}

	log.WithFields(log.Fields{
		"result": result,
	}).Debug("Exit result ")
	return result, nil
}

func readIndexEntry(file *os.File, indexPosition int64) (uint64, uint32, error) {
	msgOffsetBuff := make([]byte, INDEX_ENTRY_SIZE)
	if _, err := file.ReadAt(msgOffsetBuff, indexPosition); err != nil {
		return 0, 0, err
	}
	msgOffset := binary.LittleEndian.Uint64(msgOffsetBuff)
	msgSize := binary.LittleEndian.Uint32(msgOffsetBuff[8:])
	return msgOffset, msgSize, nil
}

// checkoutMessagefile returns a file handle to the message file with the supplied file id. The returned file handle may be shared for multiple go routines.
func (p *MessagePartition) checkoutMessagefile(fileId uint64) (*os.File, error) {
	return os.Open(p.filenameByMessageId(fileId))
}

// releaseMessagefile releases a message file handle
func (p *MessagePartition) releaseMessagefile(fileId uint64, file *os.File) {
	file.Close()
}

// checkoutIndexfile returns a file handle to the index file with the supplied file id. The returned file handle may be shared for multiple go routines.
func (p *MessagePartition) checkoutIndexfile(fileId uint64) (*os.File, error) {
	return os.Open(p.indexFilenameByMessageId(fileId))
}

// releaseIndexfile releases an index file handle
func (p *MessagePartition) releaseIndexfile(fileId uint64, file *os.File) {
	file.Close()
}

func (p *MessagePartition) firstMessageIdForFile(messageId uint64) uint64 {
	return messageId - messageId%MESSAGES_PER_FILE
}

func (p *MessagePartition) filenameByMessageId(messageId uint64) string {
	return filepath.Join(p.basedir, fmt.Sprintf("%s-%020d.msg", p.name, messageId))
}

func (p *MessagePartition) indexFilenameByMessageId(messageId uint64) string {
	return filepath.Join(p.basedir, fmt.Sprintf("%s-%020d.idx", p.name, messageId))
}
