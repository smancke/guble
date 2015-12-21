package store

import (
	"github.com/smancke/guble/guble"

	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"

	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"
)

var WriteTestFilename = "db_testfile"

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

func (kvStore *SqliteKVStore) Get(schema, key string) (value []byte, exist bool, err error) {
	entry := &kvEntry{}
	if err := kvStore.db.First(&entry, "schema = ? and key = ?", schema, key).Error; err != nil {
		if err == gorm.RecordNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}

	return entry.Value, true, nil
}

func (kvStore *SqliteKVStore) IterateKeys(schema string, keyPrefix string) chan string {
	responseChan := make(chan string, 100)

	go func() {

		rows, err := kvStore.db.Raw("select key from kv_entry where schema = ? and key LIKE ?", schema, keyPrefix+"%").
			Rows()

		if err != nil {
			guble.Err("error fetching keys from db %v", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var value string
				rows.Scan(&value)
				responseChan <- value
			}
		}
		close(responseChan)
	}()
	return responseChan
}

func (kvStore *SqliteKVStore) Delete(schema, key string) error {
	return kvStore.db.Delete(&kvEntry{Schema: schema, Key: key}).Error
}

// Opens the database file.
// If the directory does not exist, it will be created.
func (kvStore *SqliteKVStore) Open() error {
	directoryPath := filepath.Dir(kvStore.filename)
	if err := ensureWriteableDirectory(directoryPath); err != nil {
		guble.Err("error db directory not writeable %q: %q", kvStore.filename, err)
		return err
	}

	guble.Info("opening sqldb %v", kvStore.filename)
	gormdb, err := gorm.Open("sqlite3", kvStore.filename)
	if err != nil {
		guble.Err("error opening sqlite3 db %q: %q", kvStore.filename, err)
		return err
	}

	if err := gormdb.DB().Ping(); err != nil {
		guble.Err("error pinging database %q: %q", kvStore.filename, err.Error())
	} else {
		guble.Debug("can ping database %q", kvStore.filename)
	}

	//gormdb.LogMode(true)
	gormdb.DB().SetMaxIdleConns(2)
	gormdb.DB().SetMaxOpenConns(5)
	gormdb.SingularTable(true)

	if err := gormdb.AutoMigrate(&kvEntry{}).Error; err != nil {
		guble.Err("error in schema migration: %q", err)
		return err
	} else {
		guble.Debug("ensured db schema")
	}

	if !kvStore.syncOnWrite {
		guble.Info("setting db: PRAGMA synchronous = OFF")
		if err := gormdb.Exec("PRAGMA synchronous = OFF").Error; err != nil {
			guble.Err("error setting PRAGMA synchronous = OFF: %v", err)
			return err
		}
	}
	kvStore.db = &gormdb
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
		return fmt.Errorf("not a directory %v", dir)
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
