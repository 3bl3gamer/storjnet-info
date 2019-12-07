package main

import (
	"os"
	"storj3stat/server"
	"storj3stat/updater"
	"storj3stat/utils"
	"storj3stat/versions"

	"github.com/ansel1/merry"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var env = utils.Env{Val: "dev"}

var httpCmdFlags = struct {
	serverAddr string
}{}
var checkVersionsCmdFlags = struct {
	tgBotToken        string
	tgSocks5ProxyAddr string
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
	checkVersionsCmd = &cobra.Command{
		Use:   "check-versions",
		Short: "check if versions on github and version.storj.io have changed",
		RunE:  CMDCheckVersions,
	}
)

func CMDHttp(cmd *cobra.Command, args []string) error {
	return merry.Wrap(server.StartHTTPServer(httpCmdFlags.serverAddr, env))
}

func CMDUpdate(cmd *cobra.Command, args []string) error {
	return merry.Wrap(updater.StartUpdater())
}

func CMDCheckVersions(cmd *cobra.Command, args []string) error {
	return merry.Wrap(versions.CheckVersions(checkVersionsCmdFlags.tgBotToken, checkVersionsCmdFlags.tgSocks5ProxyAddr))
}

func init() {
	rootCmd.AddCommand(httpCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(checkVersionsCmd)

	flags := httpCmd.Flags()
	flags.Var(&env, "env", "evironment, dev or prod")
	flags.StringVar(&httpCmdFlags.serverAddr, "addr", "127.0.0.1:9003", "HTTP server address:port")

	flags = checkVersionsCmd.Flags()
	flags.StringVar(&checkVersionsCmdFlags.tgBotToken, "tg-bot-token", "", "TG bot API token")
	flags.StringVar(&checkVersionsCmdFlags.tgSocks5ProxyAddr, "tg-proxy", "", "SOCKS5 proxy for TG requests")
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
