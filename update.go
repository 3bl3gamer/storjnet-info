package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg"
	"google.golang.org/grpc/status"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

func NewInspector() (pb.KadInspectorClient, error) {
	ctx := context.Background()
	conn, err := transport.DialAddressInsecure(ctx, "127.0.0.1:7778")
	if err != nil {
		return nil, err
	}
	return pb.NewKadInspectorClient(conn), nil
}

type KadDataFetcher struct {
	nodeIDsChan chan storj.NodeID
	errChan     chan error
}

func (u KadDataFetcher) Add(id storj.NodeID) {
	u.nodeIDsChan <- id
}

func StartNodesKadDataFetcher(saver *KadDataSaver) *KadDataFetcher {
	fetcher := &KadDataFetcher{
		nodeIDsChan: make(chan storj.NodeID, 16),
		errChan:     make(chan error, 1),
	}

	inspector, err := NewInspector()
	if err != nil {
		fetcher.errChan <- merry.Wrap(err)
		return fetcher
	}

	go func() {
		defer close(fetcher.errChan)
		for nodeID := range fetcher.nodeIDsChan {
			resp, err := inspector.LookupNode(context.Background(), &pb.LookupNodeRequest{
				Id: nodeID.String(),
			})
			if err != nil {
				if st, ok := status.FromError(err); ok {
					if st.Message() == "node not found" {
						log.Printf("WARN: KAD: skipping %s: not found", nodeID)
					} else {
						log.Printf("WARN: KAD: skipping %s: strange message: %s", nodeID, st.Message())
					}
				} else {
					fmt.Printf("WARN: KAD: skipping %s: strange reason: %s", nodeID, err)
				}
				continue
			}
			saver.Add(resp.GetNode())
		}
	}()
	return fetcher
}

type NeighborsKadDataFetcher struct {
	errChan chan error
}

func StartNeighborsKadDataFetcher(saver *KadDataSaver) *NeighborsKadDataFetcher {
	fetcher := &NeighborsKadDataFetcher{
		errChan: make(chan error, 1),
	}

	inspector, err := NewInspector()
	if err != nil {
		fetcher.errChan <- merry.Wrap(err)
		return fetcher
	}

	go func() {
		for {
			nodes, err := inspector.FindNear(context.Background(), &pb.FindNearRequest{
				Start: storj.NodeID{},
				Limit: 100000,
			})
			if err != nil {
				//fetcher.errChan <- merry.Wrap(err)
				log.Printf("WARN: NEI: %s", err)
				continue
			}
			for _, node := range nodes.GetNodes() {
				saver.Add(node)
			}
			time.Sleep(30 * time.Second)
		}
	}()
	return fetcher
}

type OldKadDataLoader struct {
	errChan chan error
}

func StartOldKadDataLoader(db *pg.DB) *OldKadDataLoader {
	loader := &OldKadDataLoader{
		errChan: make(chan error, 1),
	}

	go func() {
		for {
			//
		}
	}()
	return loader
}
