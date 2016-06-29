package benchmarks

import (
	log "github.com/Sirupsen/logrus"
)

var logger = log.WithFields(log.Fields{
	"app":    "benchmarks",
	"module": "gcm",
})
