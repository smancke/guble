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
	Exit     bool     `arg:"-x,help: Exit after sending the commands"`
	Commands []string `arg:"positional,help: The commands to send after startup"`
	Verbose  bool     `arg:"-v,help: Display verbose server communication"`
	URL      string   `arg:"help: The websocket url to connect (ws://localhost:8080/stream/)"`
	User     string   `arg:"help: The user name to connect with (guble-cli)"`
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
	url := fmt.Sprintf("%v/user/%v", removeTrailingSlash(args.URL), args.User)
	client, err := client.Open(url, origin, 100, true)
	if err != nil {
		log.Fatal(err)
	}

	go writeLoop(client)
	go readLoop(client)

	for _, cmd := range args.Commands {
		client.WriteRawMessage([]byte(cmd))
	}
	if args.Exit {
		return
	}
	waitForTermination(func() {})
}

func loadArgs() Args {
	args := Args{
		Verbose: false,
		URL:     "ws://localhost:8080/stream/",
		User:    "guble-cli",
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
			fmt.Println(string(status.Bytes()))
			fmt.Println()
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
	if len(path) > 0 && path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}
