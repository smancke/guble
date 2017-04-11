package kvstore

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"

	"errors"
	"time"
)

const (
	responseChannelSize = 100
)

type kvEntry struct {
	Schema    string    `gorm:"primary_key" sql:"type:varchar(200)"`
	Key       string    `gorm:"primary_key" sql:"type:varchar(200)"`
	Value     []byte    `sql:"type:bytea"`
	UpdatedAt time.Time ``
}

type kvStore struct {
	db     *gorm.DB
	logger *log.Entry
}

func (store *kvStore) Stop() error {
	if store.db != nil {
		err := store.db.Close()
		store.db = nil
		return err
	}
	return nil
}

func (store *kvStore) Check() error {
	if store.db == nil {
		errorMessage := "Error: Database is not initialized (nil)"
		store.logger.Error(errorMessage)
		return errors.New(errorMessage)
	}
	if err := store.db.DB().Ping(); err != nil {
		store.logger.WithField("error", err.Error()).Error("Error pinging database")
		return err
	}
	return nil
}

func (store *kvStore) Put(schema, key string, value []byte) error {
	if err := store.Delete(schema, key); err != nil {
		return err
	}
	entry := &kvEntry{Schema: schema, Key: key, Value: value, UpdatedAt: time.Now()}
	return store.db.Create(entry).Error
}

func (store *kvStore) Get(schema, key string) ([]byte, bool, error) {
	entry := &kvEntry{}
	if err := store.db.First(&entry, "schema = ? and key = ?", schema, key).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}

	return entry.Value, true, nil
}

func (store *kvStore) Iterate(schema string, keyPrefix string) chan [2]string {
	responseC := make(chan [2]string, responseChannelSize)
	go func() {
		rows, err := store.db.Raw("select key, value from kv_entry where schema = ? and key LIKE ?", schema, keyPrefix+"%").
			Rows()
		if err != nil {
			store.logger.WithField("error", err.Error()).Error("Error fetching keys from database")
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

func (store *kvStore) IterateKeys(schema string, keyPrefix string) chan string {
	responseC := make(chan string, responseChannelSize)
	go func() {
		rows, err := store.db.Raw("select key from kv_entry where schema = ? and key LIKE ?", schema, keyPrefix+"%").
			Rows()
		if err != nil {
			store.logger.WithField("error", err.Error()).Error("Error fetching keys from database")
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

func (store *kvStore) Delete(schema, key string) error {
	return store.db.Delete(&kvEntry{Schema: schema, Key: key}).Error
}
