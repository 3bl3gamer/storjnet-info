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
)

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(importCmd)
	importCmd.AddCommand(importIDsCmd)
	importCmd.AddCommand(importKadDataCmd)
}

func CMDImportNodeIDs(cmd *cobra.Command, args []string) (err error) {
	return merry.Wrap(ImportNodeIDs(args[0]))
}

func CMDImportNodesKadData(cmd *cobra.Command, args []string) (err error) {
	return merry.Wrap(ImportNodesKadData(args[0]))
}

func CMDRun(cmd *cobra.Command, args []string) error {
	db := makePGConnection()
	nodeIDsForKadChan := make(chan storj.NodeID, 16)
	kadDataForSaveChan := make(chan *pb.Node, 16)
	kadDataForSelfChan := make(chan *pb.Node, 16)
	selfDataForSaveChan := make(chan *NodeInfoWithID, 16)
	workers := []Worker{
		StartOldKadDataLoader(db, nodeIDsForKadChan),
		StartNodesKadDataFetcher(nodeIDsForKadChan, kadDataForSaveChan),
		StartNeighborsKadDataFetcher(kadDataForSaveChan),
		StartNodesKadDataSaver(db, kadDataForSaveChan),
		//
		StartOldSelfDataLoader(db, kadDataForSelfChan),
		StartNodesSelfDataFetcher(kadDataForSelfChan, selfDataForSaveChan),
		StartNodesSelfDataSaver(db, selfDataForSaveChan),
		//
		StartNodesKadDataImporter("-", true, kadDataForSaveChan),
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

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Print(merry.Details(err))
	}
}
