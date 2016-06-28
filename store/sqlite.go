package store

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"

	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"
)

var writeTestFilename = "db_testfile"

var sqliteLogger = log.WithField("module", "kv-sqlite")

type SqliteKVStore struct {
	db          *gorm.DB
	filename    string
	syncOnWrite bool
}

func NewSqliteKVStore(filename string, syncOnWrite bool) *SqliteKVStore {
	kvStore := &SqliteKVStore{}
	kvStore.filename = filename
	kvStore.syncOnWrite = syncOnWrite
	return kvStore
}

// Open opens the database file.
// If the directory does not exist, it will be created.
func (kvStore *SqliteKVStore) Open() error {
	directoryPath := filepath.Dir(kvStore.filename)
	if err := ensureWriteableDirectory(directoryPath); err != nil {
		sqliteLogger.WithFields(log.Fields{
			"dbFilename": kvStore.filename,
			"err":        err,
		}).Error("DB Directory is not writeable")
		return err
	}

	sqliteLogger.WithField("dbFilename", kvStore.filename).Info("Opening database")

	gormdb, err := gorm.Open("sqlite3", kvStore.filename)
	if err != nil {
		sqliteLogger.WithFields(log.Fields{
			"dbFilename": kvStore.filename,
			"err":        err,
		}).Error("Error opening database")
		return err
	}

	if err := gormdb.DB().Ping(); err != nil {
		sqliteLogger.WithFields(log.Fields{
			"dbFilename": kvStore.filename,
			"err":        err,
		}).Error("Error pinging database")
	} else {
		sqliteLogger.WithField("dbFilename", kvStore.filename).Info("Ping reply from database")
	}

	gormdb.LogMode(gormLogMode)
	gormdb.SingularTable(true)
	gormdb.DB().SetMaxIdleConns(dbMaxIdleConns)
	gormdb.DB().SetMaxOpenConns(dbMaxOpenConns)

	if err := gormdb.AutoMigrate(&kvEntry{}).Error; err != nil {
		sqliteLogger.WithField("err", err).Error("Error in schema migration")
		return err
	}

	sqliteLogger.Info("Ensured database schema")

	if !kvStore.syncOnWrite {
		sqliteLogger.Info("Setting db: PRAGMA synchronous = OFF")
		if err := gormdb.Exec("PRAGMA synchronous = OFF").Error; err != nil {
			sqliteLogger.WithField("err", err).Error("Error setting PRAGMA synchronous = OFF")
			return err
		}
	}
	kvStore.db = gormdb
	return nil
}

func (kvStore *SqliteKVStore) Check() error {
	if kvStore.db == nil {
		errorMessage := "Error: Database is not initialized (nil)"
		sqliteLogger.Error(errorMessage)
		return errors.New(errorMessage)
	}
	if err := kvStore.db.DB().Ping(); err != nil {
		sqliteLogger.WithFields(log.Fields{
			"dbFilename": kvStore.filename,
			"err":        err,
		}).Error("Error pinging database")
		return err
	}
	return nil
}

//TODO Cosmin should Stop be invoked from Router, Service, or gubled?
func (kvStore *SqliteKVStore) Stop() error {
	if kvStore.db != nil {
		err := kvStore.db.Close()
		kvStore.db = nil
		return err
	}
	return nil
}

func (kvStore *SqliteKVStore) Put(schema, key string, value []byte) error {
	if err := kvStore.Delete(schema, key); err != nil {
		return err
	}
	entry := &kvEntry{Schema: schema, Key: key, Value: value, UpdatedAt: time.Now()}
	return kvStore.db.Create(entry).Error
}

func (kvStore *SqliteKVStore) Get(schema, key string) ([]byte, bool, error) {
	entry := &kvEntry{}
	if err := kvStore.db.First(&entry, "schema = ? and key = ?", schema, key).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}

	return entry.Value, true, nil
}

func (kvStore *SqliteKVStore) Iterate(schema string, keyPrefix string) chan [2]string {
	responseC := make(chan [2]string, responseChannelSize)
	go func() {
		rows, err := kvStore.db.Raw("select key, value from kv_entry where schema = ? and key LIKE ?", schema, keyPrefix+"%").
			Rows()
		if err != nil {
			kvLogger.WithField("err", err).Error("Error fetching keys from database")
		} else {
			defer rows.Close()
			for rows.Next() {
				var key, value string
				rows.Scan(&key, &value)
				responseC <- [2]string{key, value}
			}
		}
		close(responseC)
	}()
	return responseC
}

func (kvStore *SqliteKVStore) IterateKeys(schema string, keyPrefix string) chan string {
	responseC := make(chan string, responseChannelSize)
	go func() {
		rows, err := kvStore.db.Raw("select key from kv_entry where schema = ? and key LIKE ?", schema, keyPrefix+"%").
			Rows()
		if err != nil {
			kvLogger.WithField("err", err).Error("Error fetching keys from database")
		} else {
			defer rows.Close()
			for rows.Next() {
				var value string
				rows.Scan(&value)
				responseC <- value
			}
		}
		close(responseC)
	}()
	return responseC
}

func (kvStore *SqliteKVStore) Delete(schema, key string) error {
	return kvStore.db.Delete(&kvEntry{Schema: schema, Key: key}).Error
}

func ensureWriteableDirectory(dir string) error {
	dirInfo, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		dirInfo, err = os.Stat(dir)
	}

	if err != nil || !dirInfo.IsDir() {
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
