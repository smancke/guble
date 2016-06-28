package store

import (
	"time"
)

type kvEntry struct {
	Schema    string    `gorm:"primary_key"sql:"type:varchar(200)"`
	Key       string    `gorm:"primary_key"sql:"type:varchar(200)"`
	Value     []byte    `sql:"type:bytea"`
	UpdatedAt time.Time ``
}

const (
	maxIdleConns        = 2
	maxOpenConns        = 5
	responseChannelSize = 100
)
