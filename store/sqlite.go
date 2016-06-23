package store

import (
	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"

	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"
)

var WriteTestFilename = "db_testfile"

const (
	maxIdleConns        = 2
	maxOpenConns        = 5
	responseChannelSize = 100
)

var sqliteLogger = log.WithFields(log.Fields{
	"app":    "guble",
	"module": "kv-sqlite",
	"env":    "TBD"})

type kvEntry struct {
	Schema    string    `gorm:"primary_key"sql:"type:varchar(200)"`
	Key       string    `gorm:"primary_key"sql:"type:varchar(200)"`
	Value     []byte    `sql:"type:bytea"`
	UpdatedAt time.Time ``
}

type SqliteKVStore struct {
	data        map[string]map[string][]byte
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

func (kvStore *SqliteKVStore) Stop() error {
	if kvStore.db != nil {
		return kvStore.db.Close()
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
			sqliteLogger.WithFields(log.Fields{
				"err": err,
			}).Error("Error fetching keys from db")

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
			sqliteLogger.WithFields(log.Fields{
				"err": err,
			}).Error("Error fetching keys from db")
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

// Open opens the database file.
// If the directory does not exist, it will be created.
func (kvStore *SqliteKVStore) Open() error {
	directoryPath := filepath.Dir(kvStore.filename)
	if err := ensureWriteableDirectory(directoryPath); err != nil {
		sqliteLogger.WithFields(log.Fields{
			"dbFilename": kvStore.filename,
			"err":        err,
		}).Error("DB Directory not writeable")
		return err
	}

	sqliteLogger.WithFields(log.Fields{
		"dbFilename": kvStore.filename,
	}).Info("Opening sqldb")

	gormdb, err := gorm.Open("sqlite3", kvStore.filename)
	if err != nil {
		log.WithFields(log.Fields{
			"module":     "kv-sqlite",
			"dbFilename": kvStore.filename,
			"err":        err,
		}).Error("Error opening sqlite db")
		return err
	}

	if err := gormdb.DB().Ping(); err != nil {
		log.WithFields(log.Fields{
			"module":      "kv-sqlite",
			"db_filename": kvStore.filename,
			"err":         err,
		}).Error("Error pinging database")

	} else {
		log.WithFields(log.Fields{
			"module":      "kv-sqlite",
			"db_filename": kvStore.filename,
		}).Debug("Ping reply from database")
	}

	//gormdb.LogMode(true)
	gormdb.SingularTable(true)
	gormdb.DB().SetMaxIdleConns(maxIdleConns)
	gormdb.DB().SetMaxOpenConns(maxOpenConns)

	if err := gormdb.AutoMigrate(&kvEntry{}).Error; err != nil {
		log.WithFields(log.Fields{
			"module": "kv-sqlite",
			"err":    err,
		}).Error("Error in schema migration:")

		return err
	}

	log.WithFields(log.Fields{
		"module": "kv-sqlite",
	}).Debug("Ensured db schema")

	if !kvStore.syncOnWrite {
		sqliteLogger.Info("Setting db: PRAGMA synchronous = OFF")
		if err := gormdb.Exec("PRAGMA synchronous = OFF").Error; err != nil {

			sqliteLogger.WithFields(log.Fields{
				"err": err,
			}).Error("Error setting PRAGMA synchronous = OFF")
			return err
		}
	}
	kvStore.db = gormdb
	return nil
}

func (kvStore *SqliteKVStore) Check() error {
	if kvStore.db == nil {
		sqliteLogger.WithFields(log.Fields{
			"err": "Db service is null.",
		}).Error("Error Db pointer is not initialized")

		return errors.New("Db service is null.")
	}

	if err := kvStore.db.DB().Ping(); err != nil {
		sqliteLogger.WithFields(log.Fields{
			"db_filename": kvStore.filename,
			"err":         err,
		}).Error("Error pinging database")

		return err
	}

	return nil
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

	writeTest := path.Join(dir, WriteTestFilename)
	if err := ioutil.WriteFile(writeTest, []byte("writeTest"), 0644); err != nil {
		return err
	}
	if err := os.Remove(writeTest); err != nil {
		return err
	}
	return nil
}
