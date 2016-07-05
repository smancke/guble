package store

import (
	log "github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

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

	gormdb, err := gorm.Open("postgres", kvStore.config.String())
	if err != nil {
		logger.WithField("err", err).Error("Error opening database")
		return err
	}

	if err := gormdb.DB().Ping(); err != nil {
		logger.WithField("err", err).Error("Error pinging database")
	} else {
		logger.Info("Ping reply from database")
	}

	gormdb.LogMode(gormLogMode)
	gormdb.SingularTable(true)
	gormdb.DB().SetMaxIdleConns(dbMaxIdleConns)
	gormdb.DB().SetMaxOpenConns(dbMaxOpenConns)
	if err := gormdb.AutoMigrate(&kvEntry{}).Error; err != nil {
		logger.WithField("err", err).Error("Error in schema migration")
		return err
	}

	logger.Info("Ensured database schema")
	kvStore.db = gormdb
	return nil
}
