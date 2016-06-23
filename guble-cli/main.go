package main

import (
	"bufio"
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"

	"os"
	"os/signal"
	"strings"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/smancke/guble/client"
	"github.com/smancke/guble/protocol"
)

var (
	Exit     = kingpin.Flag("exit", "Exit after sending the commands").Short('x').Bool()
	Commands = kingpin.Arg("commands", "The commands to send after startup").Strings()
	Verbose  = kingpin.Flag("verbose", "Display verbose server communication").Short('v').Bool()
	URL      = kingpin.Flag("url", "The websocket url to connect to").Default("ws://localhost:8080/stream/").String()
	User     = kingpin.Flag("user", "The user name to connect with (guble-cli)").Default("guble-cli").String()
	Log      = kingpin.Flag("log", "Log level").
			Default(log.ErrorLevel.String()).
			Envar("GUBLE_LOG").
			Enum(logLevels()...)

	logger = log.WithField("app", "guble-cli")
)

func logLevels() (levels []string) {
	for _, level := range log.AllLevels {
		levels = append(levels, level.String())
	}
	return
}

// This is a minimal commandline client to connect through a websocket
func main() {
	kingpin.Parse()

	// set log level
	level, err := log.ParseLevel(*Log)
	if err != nil {
		logger.WithField("error", err).Fatal("Invalid log level")
	}
	log.SetLevel(level)

	origin := "http://localhost/"
	url := fmt.Sprintf("%v/user/%v", removeTrailingSlash(*URL), *User)
	client, err := client.Open(url, origin, 100, true)
	if err != nil {
		log.Fatal(err)
	}

	go writeLoop(client)
	go readLoop(client)

	for _, cmd := range *Commands {
		client.WriteRawMessage([]byte(cmd))
	}
	if *Exit {
		return
	}
	waitForTermination(func() {})
}

func readLoop(client client.Client) {
	for {
		select {
		case incomingMessage := <-client.Messages():
			if *Verbose {
				fmt.Println(string(incomingMessage.Bytes()))
			} else {
				fmt.Printf("%v: %v\n", incomingMessage.UserID, incomingMessage.BodyAsString())
			}
		case e := <-client.Errors():
			fmt.Println("ERROR: " + string(e.Bytes()))
		case status := <-client.StatusMessages():
			fmt.Println(string(status.Bytes()))
			fmt.Println()
		}
	}
}

func writeLoop(client client.Client) {
	shouldStop := false
	for !shouldStop {
		func() {
			defer protocol.PanicLogger()
			reader := bufio.NewReader(os.Stdin)
			text, _ := reader.ReadString('\n')
			if strings.TrimSpace(text) == "" {
				return
			}

			if strings.TrimSpace(text) == "?" || strings.TrimSpace(text) == "help" {
				printHelp()
				return
			}

			if strings.HasPrefix(text, ">") {
				fmt.Print("header: ")
				header, _ := reader.ReadString('\n')
				text += header
				fmt.Print("body: ")
				body, _ := reader.ReadString('\n')
				text += strings.TrimSpace(body)
			}

			if *Verbose {
				log.Printf("Sending: %v\n", text)
			}
			if err := client.WriteRawMessage([]byte(text)); err != nil {
				shouldStop = true

				logger.WithField("err", err).Error("Error on Writing  message")
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

func printHelp() {
	fmt.Println(`
## Commands
?           # print this info

+ /foo/bar  # subscribe to the topic /foo/bar
+ /foo 0    # read from message 0 and subscribe to the topic /foo
+ /foo 0 5  # read messages 0-5 from /foo
+ /foo -5   # read the last 5 messages and subscribe to the topic /foo

- /foo      # cancel the subscription for /foo

> /foo         # send a message to /foo
> /foo/bar 42  # send a message to /foo/bar with publisherid 42
`)
}

func removeTrailingSlash(path string) string {
	if len(path) > 1 && path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}
