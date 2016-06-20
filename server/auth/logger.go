package auth

import (
	log "github.com/Sirupsen/logrus"
)

var logger = log.WithFields(log.Fields{
	"app":    "guble",
	"module": "accessManager",
	"env":    "TBD",
})
