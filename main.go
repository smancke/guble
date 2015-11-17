package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/smancke/guble/server"

	"github.com/alexflint/go-arg"
	"github.com/caarlos0/env"
	"time"
)

type Args struct {
	Listen string `Arg:"-L,Help: [Host:]Port the address to listen on (:8080)" env:"GN_LISTEN"`
}

func main() {
	args := loadArgs()

	mux := server.NewPubSubRouter().Go()
	wshandler := server.NewWSHandler(mux, mux)
	server.StartWSServer(args.Listen, wshandler)

	waitForTermination(func() {
		mux.Stop()
		time.Sleep(time.Second * 2)
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
	log.Printf("Got singal '%v' .. exit greacefully now", <-sigc)
	callback()
	log.Printf("exit now")
	os.Exit(0)
}
