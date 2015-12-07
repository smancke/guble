package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/alexflint/go-arg"

	"github.com/smancke/guble/client"
	"github.com/smancke/guble/guble"
)

type Args struct {
	Verbose  bool     `arg:"-v,help: Display verbose server communication"`
	Url      string   `arg:"help: The websocket url to connect (ws://localhost:8080/)"`
	Topics   []string `arg:"positional,help: The topics to subscribe on connect"`
	LogInfo  bool     `arg:"--log-info,help: Log on INFO level (false)" env:"GUBLE_LOG_INFO"`
	LogDebug bool     `arg:"--log-debug,help: Log on DEBUG level (false)" env:"GUBLE_LOG_DEBUG"`
}

var args Args

// This is a minimal commandline client to connect through a websocket
func main() {
	guble.LogLevel = guble.LEVEL_ERR

	args = loadArgs()
	if args.LogInfo {
		guble.LogLevel = guble.LEVEL_INFO
	}
	if args.LogDebug {
		guble.LogLevel = guble.LEVEL_DEBUG
	}

	origin := "http://localhost/"
	client, err := client.Open(args.Url, origin, 100, true)
	if err != nil {
		log.Fatal(err)
	}

	go writeLoop(client)
	go readLoop(client)

	for _, topic := range args.Topics {
		fmt.Printf("+ %v\n", topic)
		client.Subscribe(topic)
	}
	waitForTermination(func() {})
}

func loadArgs() Args {
	args := Args{
		Verbose: false,
		Url:     "ws://localhost:8080/",
	}

	arg.MustParse(&args)
	return args
}

func readLoop(client *client.Client) {
	for {
		select {
		case incomingMessage := <-client.Messages():
			if args.Verbose {
				fmt.Println(string(incomingMessage.Bytes()))
			} else {
				fmt.Printf("%v: %v\n", incomingMessage.PublisherUserId, incomingMessage.BodyAsString())
			}
		case error := <-client.Errors():
			fmt.Println("ERROR: " + string(error.Bytes()))
		case status := <-client.StatusMessages():
			if args.Verbose {
				fmt.Println(string(status.Bytes()))
			}
		}
	}
}

func writeLoop(client *client.Client) {
	shouldStop := false
	for !shouldStop {
		func() {
			defer guble.PanicLogger()
			reader := bufio.NewReader(os.Stdin)
			text, _ := reader.ReadString('\n')

			if strings.HasPrefix(text, ">") {
				fmt.Print("header: ")
				header, _ := reader.ReadString('\n')
				text += header
				fmt.Print("body: ")
				body, _ := reader.ReadString('\n')
				text += strings.TrimSpace(body)
			}

			if args.Verbose {
				log.Printf("Sending: %v\n", text)
			}
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
