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
		Use:   "storj3stat",
		Short: "Tool for gathering storj network stats",
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

func start() error {
	db := makePGConnection()
	nodeIDsForKadChan := make(chan storj.NodeID, 16)
	kadDataForSaveChan := make(chan *pb.Node, 16)
	kadDataForSelfChan := make(chan *pb.Node, 16)
	selfDataForSaveChan := make(chan *NodeInfoWithID, 16)
	workers := []Worker{
		StartOldKadDataLoader(db, nodeIDsForKadChan),
		StartNodesKadDataFetcher(nodeIDsForKadChan, kadDataForSaveChan),
		//StartNeighborsKadDataFetcher(kadDataChan),
		StartNodesKadDataSaver(db, kadDataForSaveChan),
		//
		StartOldSelfDataLoader(db, kadDataForSelfChan),
		StartNodesSelfDataFetcher(kadDataForSelfChan, selfDataForSaveChan),
		StartNodesSelfDataSaver(db, selfDataForSaveChan),
	}
	for {
		for _, worker := range workers {
			if err := worker.PopError(); err != nil {
				return err
			}
		}
		time.Sleep(time.Second)
	}
	return nil
}

func main() {
	if err := start(); err != nil {
		log.Print(merry.Details(err))
	}
	// if err := rootCmd.Execute(); err != nil {
	// 	log.Print(merry.Details(err))
	// }
}
