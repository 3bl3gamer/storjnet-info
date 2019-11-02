package main

import (
	"flag"
	"os"
	"storj3stat/server"
	"storj3stat/utils"

	"github.com/ansel1/merry"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	var env = utils.Env{Val: "dev"}

	// Flags
	var serverAddr string
	flag.StringVar(&serverAddr, "addr", "127.0.0.1:9003", "server address:port")
	flag.Var(&env, "env", "server environment: dev or prod")
	flag.Parse()

	// Logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.ErrorStackMarshaler = func(err error) interface{} { return merry.Details(err) }
	zerolog.ErrorStackFieldName = "message" //TODO: https://github.com/rs/zerolog/issues/157
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02 15:04:05.000"})

	if err := server.StartHTTPServer(serverAddr, env); err != nil {
		panic(err)
	}
}
