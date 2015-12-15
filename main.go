package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/smancke/guble/guble"
	"github.com/smancke/guble/server"

	"github.com/alexflint/go-arg"
	"github.com/caarlos0/env"
	"time"
)

type Args struct {
	Listen   string `arg:"-l,help: [Host:]Port the address to listen on (:8080)" env:"GUBLE_LISTEN"`
	LogInfo  bool   `arg:"--log-info,help: Log on INFO level (false)" env:"GUBLE_LOG_INFO"`
	LogDebug bool   `arg:"--log-debug,help: Log on DEBUG level (false)" env:"GUBLE_LOG_DEBUG"`
}

func main() {

	args := loadArgs()
	if args.LogInfo {
		guble.LogLevel = guble.LEVEL_INFO
	}
	if args.LogDebug {
		guble.LogLevel = guble.LEVEL_DEBUG
	}

	startupServer(args)
}

func startupServer(args Args) {
	router := server.NewPubSubRouter().Go()
	messageEntry := server.NewMessageEntry(router)
	wshandlerFactory := func(wsConn server.WSConn, userId string) server.Startable {
		return server.NewWSHandler(router, messageEntry, wsConn, userId)
	}
	ws := server.StartWSServer(args.Listen, "/", wshandlerFactory)

	waitForTermination(func() {
		router.Stop()
		ws.Stop()
		time.Sleep(time.Second * 1)
	})
}

func loadArgs() Args {
	args := Args{
		Listen: ":8080",
	}

	env.Parse(&args)
	arg.MustParse(&args)
	return args
}

func waitForTermination(callback func()) {
	sigc := make(chan os.Signal)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	guble.Info("Got singal '%v' .. exit greacefully now", <-sigc)
	callback()
	guble.Info("exit now")
	os.Exit(0)
}
