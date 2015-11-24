package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/smancke/guble/client"
)

// This is a minimal commandline client to connect through a websocket
func main() {
	url := "ws://localhost:8080/"
	if len(os.Args) == 2 {
		url = os.Args[1]
	}

	fmt.Printf("connecting to %q\n", url)

	origin := "http://localhost/"
	_, err := client.Open(url, origin, 100)
	if err != nil {
		log.Fatal(err)
	}

	waitForTermination(func() {})
}

func waitForTermination(callback func()) {
	sigc := make(chan os.Signal)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("%q", <-sigc)
	callback()
	os.Exit(0)
}
