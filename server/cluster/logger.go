package cluster

import (
	log "github.com/Sirupsen/logrus"
)

var logger = log.WithFields(log.Fields{
	"app":    "guble",
	"module": "router",
	"env":    "TBD",
})
