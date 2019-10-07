package main

import (
	"os"

	"github.com/ansel1/merry"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var envMode = "dev"

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.ErrorStackMarshaler = func(err error) interface{} { return merry.Details(err) }
	zerolog.ErrorStackFieldName = "message" //TODO: https://github.com/rs/zerolog/issues/157
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02 15:04:05.000"})

	if err := StartHTTPServer("127.0.0.1:9003"); err != nil {
		panic(err)
	}
}
