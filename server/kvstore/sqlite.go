package kvstore

import (
	// use this as gorm's sqlite dialect / implementation
	_ "github.com/mattn/go-sqlite3"

	"github.com/jinzhu/gorm"

	log "github.com/Sirupsen/logrus"

	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

const (
	sqliteMaxIdleConns = 2
	sqliteMaxOpenConns = 5
	sqliteGormLogMode  = false
)

var writeTestFilename = "db_testfile"

// SqliteKVStore is a struct representing a sqlite database which embeds a kvStore.
type SqliteKVStore struct {
	*kvStore
	filename    string
	syncOnWrite bool
}

// NewSqliteKVStore returns a new configured SqliteKVStore (not opened yet).
func NewSqliteKVStore(filename string, syncOnWrite bool) *SqliteKVStore {
	return &SqliteKVStore{
		kvStore: &kvStore{logger: log.WithFields(log.Fields{
			"module":      "kv-sqlite",
			"filename":    filename,
			"syncOnWrite": syncOnWrite,
		})},
		filename:    filename,
		syncOnWrite: syncOnWrite,
	}
}

// Open opens the database file. If the directory does not exist, it will be created.
func (kvStore *SqliteKVStore) Open() error {
	directoryPath := filepath.Dir(kvStore.filename)
	if err := ensureWriteableDirectory(directoryPath); err != nil {
		kvStore.logger.WithError(err).Error("DB Directory is not writeable")
		return err
	}

	kvStore.logger.Info("Opening database")

	gormdb, err := gorm.Open("sqlite3", kvStore.filename)
	if err != nil {
		kvStore.logger.WithError(err).Error("Error opening database")
		return err
	}

	if err := gormdb.DB().Ping(); err != nil {
		kvStore.logger.WithError(err).Error("Error pinging database")
		return err
	}
	kvStore.logger.Info("Ping reply from database")

	gormdb.LogMode(sqliteGormLogMode)
	gormdb.SingularTable(true)
	gormdb.DB().SetMaxIdleConns(sqliteMaxIdleConns)
	gormdb.DB().SetMaxOpenConns(sqliteMaxOpenConns)

	if err := gormdb.AutoMigrate(&kvEntry{}).Error; err != nil {
		kvStore.logger.WithError(err).Error("Error in schema migration")
		return err
	}
	kvStore.logger.Info("Ensured database schema")

	if !kvStore.syncOnWrite {
		kvStore.logger.Info("Setting db: PRAGMA synchronous = OFF")
		if err := gormdb.Exec("PRAGMA synchronous = OFF").Error; err != nil {
			kvStore.logger.WithError(err).Error("Error setting PRAGMA synchronous = OFF")
			return err
		}
	}
	kvStore.db = gormdb
	return nil
}

func ensureWriteableDirectory(dir string) error {
	dirInfo, errStat := os.Stat(dir)
	if os.IsNotExist(errStat) {
		if errMkdir := os.MkdirAll(dir, 0755); errMkdir != nil {
			return errMkdir
		}
		dirInfo, errStat = os.Stat(dir)
	}
	if errStat != nil || !dirInfo.IsDir() {
		return fmt.Errorf("kv-sqlite: not a directory %v", dir)
	}
	writeTest := path.Join(dir, writeTestFilename)
	if err := ioutil.WriteFile(writeTest, []byte("writeTest"), 0644); err != nil {
		return err
	}
	if err := os.Remove(writeTest); err != nil {
		return err
	}
	return nil
}
