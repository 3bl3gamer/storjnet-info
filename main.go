package main

import (
	"os"
	"storjnet/core"
	"storjnet/nodes"
	"storjnet/optimizer"
	"storjnet/server"
	"storjnet/tgbot"
	"storjnet/transactions"
	"storjnet/updater"
	"storjnet/utils"
	"storjnet/utils/storjutils"
	"storjnet/versions"
	"time"

	"github.com/ansel1/merry"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var env = utils.Env{Val: "dev"}

var httpCmdFlags = struct {
	serverAddr string
}{}
var pingProxyCmdFlags = struct {
	serverAddr      string
	identityDirPath string
	mode            string
}{}
var tgBotCmdFlags = struct {
	botToken          string
	socks5ProxyAddr   string
	webhookURL        string
	webhookListenAddr string
	webhookListenPath string
}{}
var nodesCmdFlags = struct {
	satelliteAddress string
	socksProxy       string
}{}
var statNodesGroup string
var nodeLocsSnapFPath string
var transactionsFlags = struct {
	summaryStartDate string
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
	pingProxyCmd = &cobra.Command{
		Use:   "ping-proxy",
		Short: "start proxy for node dialing and pings",
		RunE:  CMDPingProxy,
	}
	updateCmd = &cobra.Command{
		Use:   "update",
		Short: "start updater (pinger, uptime checker, etc.)",
		RunE:  CMDUpdate,
	}
	tgBotCmd = &cobra.Command{
		Use:   "tg-bot",
		Short: "start TG bot",
		RunE:  CMDTGBot,
	}
	checkVersionsCmd = &cobra.Command{
		Use:   "check-versions",
		Short: "check if versions on github and version.storj.io have changed",
		RunE:  CMDCheckVersions,
	}
	fetchTransactionsCmd = &cobra.Command{
		Use:   "fetch-transactions",
		Short: "fetch STORJ transactions from etherscan.io",
		RunE:  CMDFetchTransactions,
	}
	fetchNodesCmd = &cobra.Command{
		Use:   "fetch-nodes",
		Short: "fetch some nodes from satellite",
		RunE:  CMDFetchNodes,
	}
	probeNodesCmd = &cobra.Command{
		Use:   "probe-nodes",
		Short: "start probing saved nodes and updating activity timestamp",
		RunE:  CMDProbeNodes,
	}
	statNodesCmd = &cobra.Command{
		Use:   "stat-nodes",
		Short: "generate and save nodes statistics",
		RunE:  CMDStatNodes,
	}
	snapNodeLocationsCmd = &cobra.Command{
		Use:   "snap-node-locations",
		Short: "save snapshot of nodes geolocations",
		RunE:  CMDSnapNodeLocations,
	}
	printNodeLocationsCmd = &cobra.Command{
		Use:   "print-node-locations",
		Short: "print snapshot of nodes geolocations",
		RunE:  CMDPrintNodeLocations,
	}
	optimizeDBCmd = &cobra.Command{
		Use:   "optimize-db",
		Short: "vacuum some big tables, update partitions",
		RunE:  CMDOptimizeDB,
	}
)

func CMDHttp(cmd *cobra.Command, args []string) error {
	return merry.Wrap(server.StartHTTPServer(httpCmdFlags.serverAddr, env))
}

func CMDPingProxy(cmd *cobra.Command, args []string) error {
	return merry.Wrap(storjutils.StartPingProxy(
		pingProxyCmdFlags.serverAddr, pingProxyCmdFlags.identityDirPath, pingProxyCmdFlags.mode))
}

func CMDUpdate(cmd *cobra.Command, args []string) error {
	return merry.Wrap(updater.StartUpdater())
}

func CMDTGBot(cmd *cobra.Command, args []string) error {
	var webhookConfig *tgbot.WebhookConfig
	url, addr, path := tgBotCmdFlags.webhookURL, tgBotCmdFlags.webhookListenAddr, tgBotCmdFlags.webhookListenPath
	if url != "" && addr != "" && path != "" {
		webhookConfig = &tgbot.WebhookConfig{URL: url, ListenAddr: addr, ListenPath: path}
	}
	return merry.Wrap(tgbot.StartTGBot(tgBotCmdFlags.botToken, tgBotCmdFlags.socks5ProxyAddr, webhookConfig))
}

func CMDCheckVersions(cmd *cobra.Command, args []string) error {
	return merry.Wrap(versions.CheckVersions(tgBotCmdFlags.botToken, tgBotCmdFlags.socks5ProxyAddr))
}

func CMDFetchTransactions(cmd *cobra.Command, args []string) error {
	var err error
	var startDate time.Time
	if transactionsFlags.summaryStartDate != "" {
		startDate, err = time.Parse("2006-01-02", transactionsFlags.summaryStartDate)
		if err != nil {
			return merry.Wrap(err)
		}
	}
	return merry.Wrap(transactions.FetchAndProcess(startDate))
}

func CMDFetchNodes(cmd *cobra.Command, args []string) error {
	return merry.Wrap(nodes.FetchAndProcess(nodesCmdFlags.satelliteAddress, nodesCmdFlags.socksProxy))
}

func CMDProbeNodes(cmd *cobra.Command, args []string) error {
	return merry.Wrap(nodes.StartProber())
}

func CMDStatNodes(cmd *cobra.Command, args []string) error {
	nodeStats := statNodesGroup == "all" || statNodesGroup == "nodes"
	dailyStats := statNodesGroup == "all" || statNodesGroup == "daily"
	offStats := statNodesGroup == "all" || statNodesGroup == "official"
	return merry.Wrap(nodes.SaveStats(nodeStats, dailyStats, offStats))
}

func CMDSnapNodeLocations(cmd *cobra.Command, args []string) error {
	return merry.Wrap(nodes.SaveLocsSnapshot())
}

func CMDPrintNodeLocations(cmd *cobra.Command, args []string) error {
	return merry.Wrap(nodes.PrintLocsSnapshot(nodeLocsSnapFPath))
}

func CMDOptimizeDB(cmd *cobra.Command, args []string) error {
	return merry.Wrap(optimizer.OptimizeDB())
}

func init() {
	rootCmd.AddCommand(httpCmd)
	rootCmd.AddCommand(pingProxyCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(tgBotCmd)
	rootCmd.AddCommand(checkVersionsCmd)
	rootCmd.AddCommand(fetchTransactionsCmd)
	rootCmd.AddCommand(fetchNodesCmd)
	rootCmd.AddCommand(probeNodesCmd)
	rootCmd.AddCommand(statNodesCmd)
	rootCmd.AddCommand(snapNodeLocationsCmd)
	rootCmd.AddCommand(printNodeLocationsCmd)
	rootCmd.AddCommand(optimizeDBCmd)

	flags := httpCmd.Flags()
	flags.Var(&env, "env", "evironment, dev or prod")
	flags.StringVar(&httpCmdFlags.serverAddr, "addr", "127.0.0.1:9003", "HTTP server address:port")

	flags = pingProxyCmd.Flags()
	flags.StringVar(&pingProxyCmdFlags.serverAddr, "addr", "127.0.0.1:9005", "ping proxy server address:port")
	flags.StringVar(&pingProxyCmdFlags.identityDirPath, "identity-dir", "identity", "path to dir with satellite identity")
	flags.StringVar(&pingProxyCmdFlags.mode, "mode", "http", "proxy server mode: http or udp")

	flags = tgBotCmd.Flags()
	flags.StringVar(&tgBotCmdFlags.botToken, "tg-bot-token", "", "TG bot API token")
	flags.StringVar(&tgBotCmdFlags.socks5ProxyAddr, "tg-proxy", "", "SOCKS5 proxy for TG requests")
	flags.StringVar(&tgBotCmdFlags.webhookURL, "tg-webhook-url", "", "TG webhook URL, will be sent to TG (http://example.com:8443/requests/listen/path)")
	flags.StringVar(&tgBotCmdFlags.webhookListenAddr, "tg-webhook-addr", "", "TG webhook address:port for https server")
	flags.StringVar(&tgBotCmdFlags.webhookListenPath, "tg-webhook-path", "", "TG webhook /requests/listen/path for https server")
	flags.StringVar(&core.GitHubOAuthToken, "github-oauth-token", "", "GitHub API OAuth token (optional, for increasing API req rate)")

	flags = checkVersionsCmd.Flags()
	flags.StringVar(&tgBotCmdFlags.botToken, "tg-bot-token", "", "TG bot API token")
	flags.StringVar(&tgBotCmdFlags.socks5ProxyAddr, "tg-proxy", "", "SOCKS5 proxy for TG requests")
	flags.StringVar(&core.GitHubOAuthToken, "github-oauth-token", "", "GitHub API OAuth token (optional, for increasing API req rate)")

	flags = fetchTransactionsCmd.Flags()
	flags.StringVar(&transactionsFlags.summaryStartDate, "summary-start-date", "", "start date for updating daily summaries")

	flags = fetchNodesCmd.Flags()
	flags.StringVar(&nodesCmdFlags.satelliteAddress, "satellite", "", "satellite id@address:port")
	flags.StringVar(&nodesCmdFlags.socksProxy, "socks-proxy", "", "proxy for satellite requests, address:port or address:port:user:passwd")
	fetchNodesCmd.MarkFlagRequired("satellite")

	flags = statNodesCmd.Flags()
	flags.StringVar(&statNodesGroup, "group", "all", "stats group: nodes/daily/official/all")

	flags = printNodeLocationsCmd.Flags()
	flags.StringVar(&nodeLocsSnapFPath, "file", nodes.LastFPathLabel, "path to .bin file")
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
