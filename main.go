package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/formatters/logstash"
	"github.com/smancke/guble/gubled"
	"time"
)

func main() {

	log.SetFormatter(&logstash.LogstashFormatter{TimestampFormat: time.RFC3339Nano})

	gubled.Main()

}
