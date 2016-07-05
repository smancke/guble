package store

import (
	log "github.com/Sirupsen/logrus"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

const postgresGormLogMode = false

type PostgresKVStore struct {
	*kvStore
	config PostgresConfig
}

func NewPostgresKVStore(postgresConfig PostgresConfig) *PostgresKVStore {
	return &PostgresKVStore{
		kvStore: &kvStore{logger: log.WithFields(log.Fields{"module": "kv-postgres"})},
		config:  postgresConfig,
	}
}

func (kvStore *PostgresKVStore) Open() error {
	logger := kvStore.logger.WithField("config", kvStore.config)
	logger.Info("Opening database")

	gormdb, err := gorm.Open("postgres", kvStore.config.connectionString())
	if err != nil {
		logger.WithField("err", err).Error("Error opening database")
		return err
	}

	if err := gormdb.DB().Ping(); err != nil {
		kvStore.logger.WithError(err).Error("Error pinging database")
	} else {
		kvStore.logger.Info("Ping reply from database")
	}

	gormdb.LogMode(postgresGormLogMode)
	gormdb.SingularTable(true)
	gormdb.DB().SetMaxIdleConns(kvStore.config.MaxIdleConns)
	gormdb.DB().SetMaxOpenConns(kvStore.config.MaxOpenConns)
	if err := gormdb.AutoMigrate(&kvEntry{}).Error; err != nil {
		logger.WithField("err", err).Error("Error in schema migration")
		return err
	}

	logger.Info("Ensured database schema")
	kvStore.db = gormdb
	return nil
}
