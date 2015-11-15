package main

import (
	"bufio"
	"fmt"
	"golang.org/x/net/websocket"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// This is a minimal commandline client to connect through a websocket
func main() {
	url := "ws://localhost:8080/"
	if len(os.Args) == 2 {
		url = os.Args[1]
	}

	fmt.Printf("connecting to %q\n", url)

	origin := "http://localhost/"
	ws, err := websocket.Dial(url, "", origin)
	if err != nil {
		log.Fatal(err)
	}

	go receiveLoop(ws)
	go sendLoop(ws)

	waitForTermination(func() {})
}

func receiveLoop(ws *websocket.Conn) {
	for {
		var msg = make([]byte, 512)
		var n int
		var err error
		if n, err = ws.Read(msg); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s", msg[:n])
	}
}

func sendLoop(ws *websocket.Conn) {
	for {
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		if _, err := ws.Write([]byte(text)); err != nil {
			log.Fatal(err)
		}
	}
}

func waitForTermination(callback func()) {
	sigc := make(chan os.Signal)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("%q", <-sigc)
	callback()
	os.Exit(0)
}
