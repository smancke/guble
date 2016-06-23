package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/gubled"
	"github.com/smancke/guble/gubled/config"
)

func main() {

	log.SetFormatter(&config.LogstashGubleFormatter{})

	gubled.Main()

}
