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
	postgresWithConfigLogger.Info("Opening database")

	gormdb, err := gorm.Open("postgres", kvStore.config.connectionString())
	if err != nil {
		postgresWithConfigLogger.WithField("err", err).Error("Error opening database")
		return err
	}

	if err := gormdb.DB().Ping(); err != nil {
		postgresWithConfigLogger.WithField("err", err).Error("Error pinging database")
	} else {
		postgresWithConfigLogger.Info("Ping reply from database")
	}

	gormdb.LogMode(postgresGormLogMode)
	gormdb.SingularTable(true)
	gormdb.DB().SetMaxIdleConns(kvStore.config.MaxIdleConns)
	gormdb.DB().SetMaxOpenConns(kvStore.config.MaxOpenConns)
	if err := gormdb.AutoMigrate(&kvEntry{}).Error; err != nil {
		postgresWithConfigLogger.WithField("err", err).Error("Error in schema migration")
		return err
	}
	postgresWithConfigLogger.Info("Ensured database schema")
	kvStore.gormKVStore = gormKVStore{gormdb}
	return nil
}
