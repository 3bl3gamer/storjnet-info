package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

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

func StartNodesKadDataFetcher(wg *sync.WaitGroup, nodeIDsChan chan storj.NodeID, kadDataChan chan *pb.Node) Worker {
	worker := NewSimpleWorker()

	inspector, err := NewInspector()
	if err != nil {
		worker.AddError(err)
		return worker
	}

	go func() {
		defer worker.Done()
		for nodeID := range nodeIDsChan {
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
			kadDataChan <- resp.GetNode()
		}
	}()
	return worker
}

func StartNeighborsKadDataFetcher(kadDataChan chan *pb.Node) Worker {
	worker := NewSimpleWorker()

	inspector, err := NewInspector()
	if err != nil {
		worker.AddError(err)
		return worker
	}

	go func() {
		defer worker.Done()
		for {
			resp, err := inspector.FindNear(context.Background(), &pb.FindNearRequest{
				Start: storj.NodeID{},
				Limit: 100000,
			})
			if err != nil {
				//fetcher.errChan <- merry.Wrap(err)
				log.Printf("WARN: NEI: %s", err)
				continue
			}
			nodes := resp.GetNodes()
			log.Printf("INFO: NEI: got %d neighbor(s)", len(nodes))
			for _, node := range nodes {
				kadDataChan <- node
			}
			time.Sleep(3 * time.Second)
		}
	}()
	return worker
}

func StartOldKadDataLoader(db *pg.DB, nodeIDsChan chan storj.NodeID) Worker {
	worker := NewSimpleWorker()

	go func() {
		defer worker.Done()
		ids := make([]storj.NodeID, 10)
		for {
			_, err := db.Query(&ids, "SELECT id FROM storj3_nodes ORDER BY kad_updated_at ASC NULLS FIRST LIMIT 10")
			if err != nil {
				worker.AddError(err)
				return
			}
			for _, id := range ids {
				nodeIDsChan <- id
			}
		}
	}()
	return worker
}
