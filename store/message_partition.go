package store

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

var MAGIC_NUMBER = []byte{42, 249, 180, 108, 82, 75, 222, 182}

var FILE_FORMAT_VERSION = []byte{1}

var MESSAGES_PER_FILE = uint64(10000)

type MessagePartition struct {
	basedir                 string
	name                    string
	appendFile              *os.File
	appendIndexFile         *os.File
	appendFirstId           uint64
	appendLastId            uint64
	appendFileWritePosition uint64
}

func NewMessagePartition(basedir string, storeName string) *MessagePartition {
	return &MessagePartition{
		basedir: basedir,
		name:    storeName,
	}
}

func (s *MessagePartition) closeAppendFiles() error {
	if s.appendFile != nil {
		if err := s.appendFile.Close(); err != nil {
			if s.appendIndexFile != nil {
				defer s.appendIndexFile.Close()
			}
			return err
		}
	}

	if s.appendIndexFile != nil {
		return s.appendIndexFile.Close()
	}
	return nil
}

func (s *MessagePartition) createNextAppendFiles(msgId uint64) error {

	firstMessageIdForFile := s.firstMessageIdForFile(msgId)

	file, err := os.OpenFile(s.filenameByMessageId(firstMessageIdForFile), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// write file header on new files
	if stat, _ := file.Stat(); stat.Size() == 0 {
		s.appendFileWritePosition = uint64(stat.Size())

		_, err = file.Write(MAGIC_NUMBER)
		if err != nil {
			return err
		}

		_, err = file.Write(FILE_FORMAT_VERSION)
		if err != nil {
			return err
		}
	}

	index, errIndex := os.OpenFile(s.indexFilenameByMessageId(firstMessageIdForFile), os.O_RDWR|os.O_CREATE, 0666)
	if errIndex != nil {
		defer file.Close()
		defer os.Remove(file.Name())
		return err
	}

	s.appendFile = file
	s.appendIndexFile = index
	s.appendFirstId = firstMessageIdForFile
	s.appendLastId = firstMessageIdForFile + MESSAGES_PER_FILE - 1
	stat, err := file.Stat()
	s.appendFileWritePosition = uint64(stat.Size())
	return nil
}

func (s *MessagePartition) Close() error {
	return s.closeAppendFiles()
}

func (s *MessagePartition) Store(msgId uint64, msg []byte) error {
	if msgId > s.appendLastId ||
		s.appendFile == nil ||
		s.appendIndexFile == nil {

		if err := s.closeAppendFiles(); err != nil {
			return err
		}
		if err := s.createNextAppendFiles(msgId); err != nil {
			return err
		}
	}

	// write the message size and the message id 32bit and 64 bit
	sizeAndId := make([]byte, 12)
	binary.LittleEndian.PutUint32(sizeAndId, uint32(len(msg)))
	binary.LittleEndian.PutUint64(sizeAndId[4:], msgId)
	if _, err := s.appendFile.Write(sizeAndId); err != nil {
		return err
	}

	// write the message
	if _, err := s.appendFile.Write(msg); err != nil {
		return err
	}

	// write the index entry to the index file
	indexPosition := int64(12 * (msgId % MESSAGES_PER_FILE))
	messageOffset := s.appendFileWritePosition + uint64(len(sizeAndId))
	messageOffsetBuff := make([]byte, 12)
	binary.LittleEndian.PutUint64(messageOffsetBuff, messageOffset)
	binary.LittleEndian.PutUint32(messageOffsetBuff[8:], uint32(len(msg)))

	if _, err := s.appendIndexFile.WriteAt(messageOffsetBuff, indexPosition); err != nil {
		return err
	}

	s.appendFileWritePosition += uint64(len(sizeAndId) + len(msg))
	return nil
}

// fetch a set of messages
func (s *MessagePartition) Fetch(req FetchRequest) {
	go func() {
		fetchList, err := s.calculateFetchList(req)
		if err != nil {
			req.ErrorCallback <- err
			return
		}

		err = s.fetchByFetchlist(fetchList, req.MessageC)
		if err != nil {
			req.ErrorCallback <- err
			return
		}
		close(req.MessageC)
	}()
}

// fetch the messages in the supplied fetchlist and send them to the channel
func (s *MessagePartition) fetchByFetchlist(fetchList []fetchEntry, messageC chan []byte) error {
	var fileId uint64
	var file *os.File
	var err error
	for _, f := range fetchList {
		// ensure, that we read on the correct file
		if file == nil || fileId != f.fileId {
			file, err = s.checkoutMessagefile(f.fileId)
			if err != nil {
				return err
			}
			defer s.releaseMessagefile(f.fileId, file)
			fileId = f.fileId
		}

		msg := make([]byte, f.size, f.size)
		_, err = file.ReadAt(msg, f.offset)
		if err != nil {
			return err
		}
		messageC <- msg
	}
	return nil
}

type fetchEntry struct {
	messageId uint64
	fileId    uint64
	offset    int64
	size      int
}

// returns a list of fetchEntry records for all message in the fetch request.
func (s *MessagePartition) calculateFetchList(req FetchRequest) ([]fetchEntry, error) {
	if req.Direction == 0 {
		req.Direction = 1
	}
	nextId := req.StartId
	result := make([]fetchEntry, 0, req.Count)
	var file *os.File
	var fileId uint64
	for len(result) < req.Count && nextId >= 0 {

		nextFileId := s.firstMessageIdForFile(nextId)

		// ensure, that we read on the correct file
		if file == nil || nextFileId != fileId {
			var err error
			file, err = s.checkoutIndexfile(nextFileId)
			if err != nil {
				if os.IsNotExist(err) {
					return result, nil
				}
				return nil, err
			}
			defer s.releaseIndexfile(fileId, file)
			fileId = nextFileId
		}

		indexPosition := int64(12 * (nextId % MESSAGES_PER_FILE))
		msgOffsetBuff := make([]byte, 12)

		if stat, err := file.Stat(); err != nil {
			return nil, err
		} else if indexPosition < stat.Size() {

			if _, err := file.ReadAt(msgOffsetBuff, indexPosition); err != nil {
				return nil, err
			}
			msgOffset := binary.LittleEndian.Uint64(msgOffsetBuff)
			msgSize := binary.LittleEndian.Uint32(msgOffsetBuff[8:])

			if msgOffset != uint64(0) { // only append, if the message exists
				result = append(result, fetchEntry{
					messageId: nextId,
					fileId:    fileId,
					offset:    int64(msgOffset),
					size:      int(msgSize),
				})
			}
		}

		nextId += uint64(req.Direction)
	}
	return result, nil
}

// Return a file handle to the message file with the supplied file id.
// The returned file handle may be shared for multiple go routines.
func (s *MessagePartition) checkoutMessagefile(fileId uint64) (*os.File, error) {
	return os.Open(s.filenameByMessageId(fileId))
}

// Release a message file handle
func (s *MessagePartition) releaseMessagefile(fileId uint64, file *os.File) {
	file.Close()
}

// Return a file handle to the index file with the supplied file id.
// The returned file handle may be shared for multiple go routines.
func (s *MessagePartition) checkoutIndexfile(fileId uint64) (*os.File, error) {
	return os.Open(s.indexFilenameByMessageId(fileId))
}

// Release an index file handle
func (s *MessagePartition) releaseIndexfile(fileId uint64, file *os.File) {
	file.Close()
}

func (s *MessagePartition) getSortedStartNumbersFromFiles() []uint64 {
	return []uint64{}
}

func (s *MessagePartition) firstMessageIdForFile(messageId uint64) uint64 {
	return messageId - messageId%MESSAGES_PER_FILE
}

func (s *MessagePartition) filenameByMessageId(messageId uint64) string {
	return filepath.Join(s.basedir, fmt.Sprintf("%s-%020d.msg", s.name, messageId))
}

func (s *MessagePartition) indexFilenameByMessageId(messageId uint64) string {
	return filepath.Join(s.basedir, fmt.Sprintf("%s-%020d.idx", s.name, messageId))
}
