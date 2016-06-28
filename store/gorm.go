package store

import (
	log "github.com/Sirupsen/logrus"

	"time"
)

type kvEntry struct {
	Schema    string    `gorm:"primary_key"sql:"type:varchar(200)"`
	Key       string    `gorm:"primary_key"sql:"type:varchar(200)"`
	Value     []byte    `sql:"type:bytea"`
	UpdatedAt time.Time ``
}

const (
	gormLogMode         = false
	dbMaxIdleConns      = 2
	dbMaxOpenConns      = 5
	responseChannelSize = 100
)

var kvLogger = log.WithField("module", "kv-gorm")
