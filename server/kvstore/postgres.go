package kvstore

import (
	log "github.com/Sirupsen/logrus"

	"github.com/jinzhu/gorm"

	// use gorm's postgres dialect
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

const postgresGormLogMode = false

// PostgresKVStore extends a gorm-based kvStore with a Postgresql-specific configuration.
type PostgresKVStore struct {
	*kvStore
	config PostgresConfig
}

// NewPostgresKVStore returns a new configured PostgresKVStore (not opened yet).
func NewPostgresKVStore(postgresConfig PostgresConfig) *PostgresKVStore {
	return &PostgresKVStore{
		kvStore: &kvStore{logger: log.WithFields(log.Fields{"module": "kv-postgres"})},
		config:  postgresConfig,
	}
}

// Open a connection to Postgresql database, or return an error.
func (kvStore *PostgresKVStore) Open() error {
	logger := kvStore.logger.WithField("config", kvStore.config)
	logger.Info("Opening database")

	gormdb, err := gorm.Open("postgres", kvStore.config.connectionString())
	if err != nil {
		logger.WithField("err", err).Error("Error opening database")
		return err
	}

	if err := gormdb.DB().Ping(); err != nil {
		kvStore.logger.WithField("error", err.Error()).Error("Error pinging database")
	} else {
		kvStore.logger.Info("Ping reply from database")
	}

	gormdb.LogMode(postgresGormLogMode)
	gormdb.SingularTable(true)
	//TODO MARIAN REMOVE THIS AFTER BUG
	gormdb.DB().SetMaxIdleConns(-1)
	gormdb.DB().SetMaxOpenConns(kvStore.config.MaxOpenConns)

	//TODO MARIAN maybe config
	//gormdb.DB().SetConnMaxLifetime(2 * time.Minute)
	if err := gormdb.AutoMigrate(&kvEntry{}).Error; err != nil {
		logger.WithField("err", err).Error("Error in schema migration")
		return err
	}

	logger.Info("Ensured database schema")
	kvStore.db = gormdb
	return nil
}
