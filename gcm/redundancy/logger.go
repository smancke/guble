package redundancy

import log "github.com/Sirupsen/logrus"

var logger = log.WithFields(log.Fields{
	"app":    "redundancy",
	"module": "gcm",
})
