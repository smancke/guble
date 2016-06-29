package store

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"

	"errors"
)

const (
	postgresMaxIdleConns = 4
	postgresMaxOpenConns = 10
)

var postgresLogger = log.WithField("module", "kv-postgres")

type PostgresKVStore struct {
	gormKVStore
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
	kvStore.gormKVStore = gormKVStore{gormdb}
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
