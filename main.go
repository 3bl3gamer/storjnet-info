package main

import (
	"os"
	"storj3stat/server"
	"storj3stat/updater"
	"storj3stat/utils"

	"github.com/ansel1/merry"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var env = utils.Env{Val: "dev"}

var runFlags = struct {
	serverAddr string
}{}

var (
	rootCmd = &cobra.Command{
		Use:   os.Args[0],
		Short: "Tool for checking Storj nodes stats",
		// SilenceUsage: true,
	}
	httpCmd = &cobra.Command{
		Use:   "http",
		Short: "start HTTP-server",
		RunE:  CMDHttp,
	}
	updateCmd = &cobra.Command{
		Use:   "update",
		Short: "start updater (pinger, uptime checker, etc.)",
		RunE:  CMDUpdate,
	}
)

func CMDHttp(cmd *cobra.Command, args []string) error {
	return merry.Wrap(server.StartHTTPServer(runFlags.serverAddr, env))
}

func CMDUpdate(cmd *cobra.Command, args []string) error {
	return merry.Wrap(updater.StartUpdater())
}

func init() {
	rootCmd.AddCommand(httpCmd)
	rootCmd.AddCommand(updateCmd)

	flags := httpCmd.Flags()
	flags.Var(&env, "env", "evironment, dev or prod")
	flags.StringVar(&runFlags.serverAddr, "addr", "127.0.0.1:9003", "HTTP server address:port")
}

func main() {
	// Logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.ErrorStackMarshaler = func(err error) interface{} { return merry.Details(err) }
	zerolog.ErrorStackFieldName = "message" //TODO: https://github.com/rs/zerolog/issues/157
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02 15:04:05.000"})

	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Msg(merry.Details(err))
	}
}
