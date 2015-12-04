package main

import (
	"bufio"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/smancke/guble/client"
	"github.com/smancke/guble/guble"
	"strings"
)

// This is a minimal commandline client to connect through a websocket
func main() {
	guble.LogLevel = guble.LEVEL_INFO

	url := "ws://localhost:8080/"
	if len(os.Args) == 2 {
		url = os.Args[1]
	}

	origin := "http://localhost/"
	client, err := client.Open(url, origin, 100, true)
	if err != nil {
		log.Fatal(err)
	}

	go writeLoop(client)
	waitForTermination(func() {})
}

func writeLoop(client *client.Client) {
	shouldStop := false
	for !shouldStop {
		func() {
			defer guble.PanicLogger()
			reader := bufio.NewReader(os.Stdin)
			text, _ := reader.ReadString('\n')

			if strings.HasPrefix(text, "send") {
				header, _ := reader.ReadString('\n')
				text += header
				body, _ := reader.ReadString('\n')
				text += strings.TrimSpace(body)
			}

			log.Printf("Sending: %v\n", text)
			if err := client.WriteRawMessage([]byte(text)); err != nil {
				shouldStop = true
				guble.Err(err.Error())
			}
		}()
	}
}

func waitForTermination(callback func()) {
	sigc := make(chan os.Signal)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("%q", <-sigc)
	callback()
	os.Exit(0)
}
