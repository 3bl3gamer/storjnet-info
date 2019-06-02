package main

import (
	"log"
	"time"

	"github.com/ansel1/merry"
	"github.com/spf13/cobra"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

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
)

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(importCmd)
	importCmd.AddCommand(importIDsCmd)
	importCmd.AddCommand(importKadDataCmd)
	rootCmd.AddCommand(saveStatsCmd)
}

func CMDImportNodeIDs(cmd *cobra.Command, args []string) (err error) {
	return merry.Wrap(ImportNodeIDs(args[0]))
}

func CMDImportNodesKadData(cmd *cobra.Command, args []string) (err error) {
	return merry.Wrap(ImportNodesKadData(args[0]))
}

func CMDRun(cmd *cobra.Command, args []string) error {
	db := makePGConnection()
	gdb, err := makeGeoIPConnection()
	if err != nil {
		return merry.Wrap(err)
	}

	nodeIDsForKadChan := make(chan storj.NodeID, 16)
	kadDataRawChan := make(chan *pb.Node, 16)
	kadDataForSaveChan := make(chan *KadDataExt, 16)
	kadDataForSelfChan := make(chan *pb.Node, 16)
	selfDataForSaveChan := make(chan *NodeInfoExt, 16)

	workers := []Worker{
		StartOldKadDataLoader(db, nodeIDsForKadChan),
		StartNodesKadDataFetcher(nodeIDsForKadChan, kadDataRawChan),
		StartNeighborsKadDataFetcher(kadDataRawChan),
		StartLocationSearcher(gdb, kadDataRawChan, kadDataForSaveChan),
		StartNodesKadDataSaver(db, kadDataForSaveChan),
		//
		StartOldSelfDataLoader(db, kadDataForSelfChan),
		StartNodesSelfDataFetcher(kadDataForSelfChan, selfDataForSaveChan),
		StartNodesSelfDataSaver(db, selfDataForSaveChan),
		//
		StartNodesKadDataImporter("-", true, kadDataRawChan),
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

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Print(merry.Details(err))
	}
}
