package store

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

const MESSAGES_PER_FILE = 10000

type FileMessageStore struct {
	basedir string
	name    string
	//allFiles                map[uint64]string // first messageid -> filename
	appendFile              *os.File
	appendIndexFile         *os.File
	appendFirstId           uint64
	appendLastId            uint64
	appendFileWritePosition uint64
}

func NewFileMessageStore(basedir string, storeName string) *FileMessageStore {
	return &FileMessageStore{
		basedir: basedir,
		name:    storeName,
	}
}

func (s *FileMessageStore) closeAppendFiles() error {
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

func (s *FileMessageStore) createNextAppendFiles(msgId uint64) error {

	firstMessageIdForFile := s.firstMessageIdForFile(msgId)

	file, err := os.Create(s.filenameByMessageId(firstMessageIdForFile))
	if err != nil {
		return err
	}

	index, errIndex := os.Create(s.indexFilenameByMessageId(firstMessageIdForFile))
	if errIndex != nil {
		defer file.Close()
		defer os.Remove(file.Name())
		return err
	}

	s.appendFile = file
	s.appendIndexFile = index
	s.appendFirstId = firstMessageIdForFile
	s.appendLastId = firstMessageIdForFile + MESSAGES_PER_FILE - 1
	s.appendFileWritePosition = 0
	return nil
}

func (s *FileMessageStore) Close() error {
	return s.closeAppendFiles()
}

func (s *FileMessageStore) Store(msgId uint64, msg []byte) error {
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
	mi := &mIndex{
		MessageId:  msgId,
		FileOffset: s.appendFileWritePosition + uint64(len(sizeAndId)),
	}

	if _, err := s.appendIndexFile.Write(mi.Bytes()); err != nil {
		return err
	}

	s.appendFileWritePosition += uint64(len(sizeAndId) + len(msg))
	return nil
}

// fetch a set of messages
func (s *FileMessageStore) Fetch(FetchRequest) {

}

func (s *FileMessageStore) firstMessageIdForFile(messageId uint64) uint64 {
	return messageId - messageId%MESSAGES_PER_FILE
}

func (s *FileMessageStore) filenameByMessageId(messageId uint64) string {
	return filepath.Join(s.basedir, fmt.Sprintf("%s-%020d.msg", s.name, messageId))
}

func (s *FileMessageStore) indexFilenameByMessageId(messageId uint64) string {
	return filepath.Join(s.basedir, fmt.Sprintf("%s-%020d.idx", s.name, messageId))
}

const INDEX_SIZE_BYTES = 17

type mIndex struct {
	MessageId  uint64
	FileOffset uint64
	Deleted    bool
}

func readMIndex(b []byte) *mIndex {
	return &mIndex{
		MessageId:  binary.LittleEndian.Uint64(b),
		FileOffset: binary.LittleEndian.Uint64(b[8:]),
		Deleted:    b[16] == 1,
	}
}

func (mindex *mIndex) writeTo(b []byte) {
	binary.LittleEndian.PutUint64(b, mindex.MessageId)
	binary.LittleEndian.PutUint64(b[8:], mindex.FileOffset)
	if mindex.Deleted {
		b[16] = 1
	} else {
		b[16] = 0
	}
}

func (mindex *mIndex) Bytes() []byte {
	b := make([]byte, INDEX_SIZE_BYTES)
	mindex.writeTo(b)
	return b
}
