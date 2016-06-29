package store

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"

	"errors"
	"time"
)

const (
	postgresMaxIdleConns = 4
	postgresMaxOpenConns = 10
)

var postgresLogger = log.WithField("module", "kv-postgres")

type PostgresKVStore struct {
	db     *gorm.DB
	config PostgresConfig
}

func NewPostgresKVStore(postgresConfig PostgresConfig) *PostgresKVStore {
	postgresKVStore := &PostgresKVStore{}
	postgresKVStore.config = postgresConfig
	return postgresKVStore
}

func (kvStore *PostgresKVStore) Open() error {
	postgresWithConfigLogger := postgresLogger.WithField("config", kvStore.config)
	postgresWithConfigLogger.Info("Opening database")

	gormdb, err := gorm.Open("postgres", kvStore.config.String())
	if err != nil {
		postgresWithConfigLogger.WithField("err", err).Error("Error opening database")
		return err
	}

	if err := gormdb.DB().Ping(); err != nil {
		postgresWithConfigLogger.WithField("err", err).Error("Error pinging database")
	} else {
		postgresWithConfigLogger.Info("Ping reply from database")
	}

	gormdb.LogMode(gormLogMode)
	gormdb.SingularTable(true)
	gormdb.DB().SetMaxIdleConns(postgresMaxIdleConns)
	gormdb.DB().SetMaxOpenConns(postgresMaxOpenConns)
	if err := gormdb.AutoMigrate(&kvEntry{}).Error; err != nil {
		postgresWithConfigLogger.WithField("err", err).Error("Error in schema migration")
		return err
	}
	postgresWithConfigLogger.Info("Ensured database schema")
	kvStore.db = gormdb
	return nil
}

//TODO Cosmin should Stop be invoked from Router, Service, or gubled?
func (kvStore *PostgresKVStore) Stop() error {
	if kvStore.db != nil {
		err := kvStore.db.Close()
		kvStore.db = nil
		return err
	}
	return nil
}

func (kvStore *PostgresKVStore) Check() error {
	if kvStore.db == nil {
		errorMessage := "Error: Database is not initialized (nil)"
		postgresLogger.WithField("config", kvStore.config).Error(errorMessage)
		return errors.New(errorMessage)
	}
	if err := kvStore.db.DB().Ping(); err != nil {
		postgresLogger.WithFields(log.Fields{
			"config": kvStore.config,
			"err":    err,
		}).Error("Error pinging database")
		return err
	}
	return nil
}

func (kvStore *PostgresKVStore) Put(schema, key string, value []byte) error {
	if err := kvStore.Delete(schema, key); err != nil {
		return err
	}
	entry := &kvEntry{Schema: schema, Key: key, Value: value, UpdatedAt: time.Now()}
	return kvStore.db.Create(entry).Error
}

func (kvStore *PostgresKVStore) Get(schema, key string) ([]byte, bool, error) {
	entry := &kvEntry{}
	if err := kvStore.db.First(&entry, "schema = ? and key = ?", schema, key).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	return entry.Value, true, nil
}

func (kvStore *PostgresKVStore) Iterate(schema string, keyPrefix string) chan [2]string {
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

func (kvStore *PostgresKVStore) IterateKeys(schema string, keyPrefix string) chan string {
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

func (kvStore *PostgresKVStore) Delete(schema, key string) error {
	return kvStore.db.Delete(&kvEntry{Schema: schema, Key: key}).Error
}
