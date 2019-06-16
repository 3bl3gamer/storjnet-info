package main

import (
	"log"
	"time"

	"github.com/ansel1/merry"
	"github.com/spf13/cobra"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

var envMode = "dev"

var (
	rootCmd = &cobra.Command{
		Use:          "storj3stat",
		Short:        "Tool for gathering storj network stats",
		SilenceUsage: true,
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "start gathering and updating storj nodes data",
		RunE:  CMDRun,
	}
	importCmd = &cobra.Command{
		Use:   "import",
		Short: "import nodes data",
	}
	importIDsCmd = &cobra.Command{
		Use:   "ids",
		Short: "import raw node IDs",
		Args:  cobra.MinimumNArgs(1),
		RunE:  CMDImportNodeIDs,
	}
	importKadDataCmd = &cobra.Command{
		Use:   "kad",
		Short: "import nodes kad data",
		Args:  cobra.MinimumNArgs(1),
		RunE:  CMDImportNodesKadData,
	}
	saveStatsCmd = &cobra.Command{
		Use:   "save-stats",
		Short: "save nodes statistics snapshot",
		RunE:  CMDSaveStats,
	}
	startHTTPServerCmd = &cobra.Command{
		Use:   "start-http-server",
		Short: "start serving stats via http",
		RunE:  CMDStartHTTPServer,
	}
)

var runFlags = struct {
	startDelay          int64
	kadImportRecentSkip int

	idsLoadChunkSize     int
	kadFetchRoutines     int
	kadNeighborsInterval int
	kadSaveChunkSize     int

	kadLoadChunkSize  int
	selfFetchRoutines int
	selfSaveChunkSize int
}{}

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(importCmd)
	importCmd.AddCommand(importIDsCmd)
	importCmd.AddCommand(importKadDataCmd)
	rootCmd.AddCommand(saveStatsCmd)
	rootCmd.AddCommand(startHTTPServerCmd)

	flags := startHTTPServerCmd.Flags()
	flags.StringVar(&envMode, "env", envMode, "evironment, dev or prod")

	flags = runCmd.Flags()
	flags.Int64Var(&runFlags.startDelay, "start-delay", 0, "delay in seconds before storagenode connection attempt")
	flags.IntVar(&runFlags.kadImportRecentSkip, "kad-import-recent-skip", 32, "")

	flags.IntVar(&runFlags.idsLoadChunkSize, "ids-load-chunk-size", 10, "")
	flags.IntVar(&runFlags.kadFetchRoutines, "kad-fetch-routines", 8, "")
	flags.IntVar(&runFlags.kadNeighborsInterval, "kad-neighbors-interval", 30, "")
	flags.IntVar(&runFlags.kadSaveChunkSize, "kad-save-chunk-size", 10, "")

	flags.IntVar(&runFlags.kadLoadChunkSize, "kad-load-chunk-size", 10, "")
	flags.IntVar(&runFlags.selfFetchRoutines, "self-fetch-routines", 8, "")
	flags.IntVar(&runFlags.selfSaveChunkSize, "self-save-chunk-size", 10, "")
}

func CMDImportNodeIDs(cmd *cobra.Command, args []string) (err error) {
	return merry.Wrap(ImportNodeIDs(args[0]))
}

func CMDImportNodesKadData(cmd *cobra.Command, args []string) (err error) {
	return merry.Wrap(ImportNodesKadData(args[0]))
}

func CMDRun(cmd *cobra.Command, args []string) error {
	if runFlags.startDelay > 0 {
		time.Sleep(time.Duration(runFlags.startDelay) * time.Second)
	}

	db := makePGConnection()
	gdb, err := makeGeoIPConnection()
	if err != nil {
		return merry.Wrap(err)
	}

	nodeIDsForKadChan := make(chan storj.NodeID, 16)
	kadDataRawChan := make(chan *pb.Node, 16)
	kadDataForSaveChan := make(chan *KadDataExt, 16)

	kadDataForSelfChan := make(chan *SelfUpdate_Kad, 16)
	selfDataForSaveChan := make(chan *SelfUpdate_Self, 16)

	workers := []Worker{
		StartOldKadDataLoader(db, nodeIDsForKadChan, runFlags.idsLoadChunkSize),
		StartNodesKadDataFetcher(nodeIDsForKadChan, kadDataRawChan, runFlags.kadFetchRoutines),
		StartNeighborsKadDataFetcher(kadDataRawChan, runFlags.kadNeighborsInterval),
		StartLocationSearcher(gdb, kadDataRawChan, kadDataForSaveChan),
		StartNodesKadDataSaver(db, kadDataForSaveChan, runFlags.kadSaveChunkSize),
		//
		StartOldSelfDataLoader(db, kadDataForSelfChan, runFlags.kadLoadChunkSize),
		StartNodesSelfDataFetcher(kadDataForSelfChan, selfDataForSaveChan, runFlags.selfFetchRoutines),
		StartNodesSelfDataSaver(db, selfDataForSaveChan, runFlags.selfSaveChunkSize),
		//
		StartNodesKadDataImporter("-", true, kadDataRawChan, runFlags.kadImportRecentSkip),
	}
	for {
		for _, worker := range workers {
			if err := worker.PopError(); err != nil {
				return err
			}
		}
		time.Sleep(time.Second)
	}
}

func CMDSaveStats(cmd *cobra.Command, args []string) error {
	db := makePGConnection()
	return merry.Wrap(SaveGlobalNodesStats(db))
}

func CMDStartHTTPServer(cmd *cobra.Command, args []string) error {
	return merry.Wrap(StartHTTPServer("0.0.0.0:9002"))
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Print(merry.Details(err))
	}
}
