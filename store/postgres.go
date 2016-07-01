package store

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

const postgresGormLogMode = false

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
	postgresWithConfigLogger.Info("Opening postgres database")

	gormdb, err := gorm.Open("postgres", kvStore.config.connectionString())
	if err != nil {
		postgresWithConfigLogger.WithError(err).Error("Error opening postgres database")
		return err
	}

	if err := gormdb.DB().Ping(); err != nil {
		postgresWithConfigLogger.WithError(err).Error("Error pinging postgres database")
	}

	gormdb.LogMode(postgresGormLogMode)
	gormdb.SingularTable(true)
	gormdb.DB().SetMaxIdleConns(kvStore.config.MaxIdleConns)
	gormdb.DB().SetMaxOpenConns(kvStore.config.MaxOpenConns)
	if err := gormdb.AutoMigrate(&kvEntry{}).Error; err != nil {
		postgresWithConfigLogger.WithError(err).Error("Error in postgres schema migration")
		return err
	}
	kvStore.gormKVStore = gormKVStore{gormdb}
	return nil
}
