package store

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"

	"errors"
	"time"
)

type kvEntry struct {
	Schema    string    `gorm:"primary_key"sql:"type:varchar(200)"`
	Key       string    `gorm:"primary_key"sql:"type:varchar(200)"`
	Value     []byte    `sql:"type:bytea"`
	UpdatedAt time.Time ``
}

const (
	responseChannelSize = 100
)

var gormLogger = log.WithField("module", "kv-gorm")

type gormKVStore struct {
	db *gorm.DB
}

func (kvStore *gormKVStore) Put(schema, key string, value []byte) error {
	if err := kvStore.Delete(schema, key); err != nil {
		return err
	}
	entry := &kvEntry{Schema: schema, Key: key, Value: value, UpdatedAt: time.Now()}
	return kvStore.db.Create(entry).Error
}

func (kvStore *gormKVStore) Get(schema, key string) ([]byte, bool, error) {
	entry := &kvEntry{}
	if err := kvStore.db.First(&entry, "schema = ? and key = ?", schema, key).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	return entry.Value, true, nil
}

func (kvStore *gormKVStore) Iterate(schema string, keyPrefix string) chan [2]string {
	responseC := make(chan [2]string, responseChannelSize)
	go func() {
		rows, err := kvStore.db.Raw("select key, value from kv_entry where schema = ? and key LIKE ?", schema, keyPrefix+"%").
			Rows()
		if err != nil {
			gormLogger.WithField("err", err).Error("Error fetching keys from database")
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

func (kvStore *gormKVStore) IterateKeys(schema string, keyPrefix string) chan string {
	responseC := make(chan string, responseChannelSize)
	go func() {
		rows, err := kvStore.db.Raw("select key from kv_entry where schema = ? and key LIKE ?", schema, keyPrefix+"%").
			Rows()
		if err != nil {
			gormLogger.WithField("err", err).Error("Error fetching keys from database")
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

func (kvStore *gormKVStore) Delete(schema, key string) error {
	return kvStore.db.Delete(&kvEntry{Schema: schema, Key: key}).Error
}

//TODO Cosmin should Stop be invoked from somewhere in our code - e.g. "service" ?
func (kvStore *gormKVStore) Stop() error {
	if kvStore.db != nil {
		err := kvStore.db.Close()
		kvStore.db = nil
		return err
	}
	return nil
}

func (kvStore *gormKVStore) Check() error {
	if kvStore.db == nil {
		errorMessage := "Error: Database is not initialized (nil)"
		gormLogger.Error(errorMessage)
		return errors.New(errorMessage)
	}
	if err := kvStore.db.DB().Ping(); err != nil {
		gormLogger.WithField("err", err).Error("Error pinging database")
		return err
	}
	return nil
}
