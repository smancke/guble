package testintegration

import (
	log "github.com/Sirupsen/logrus"
)

var logger = log.WithFields(log.Fields{
	"app":    "integrationtest",
	"module": "cluster",
})
