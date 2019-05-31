package main

import (
	"context"
	"log"
	"sync/atomic"
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

func StartNodesKadDataFetcher(nodeIDsChan chan storj.NodeID, kadDataChan chan *pb.Node) Worker {
	routinesCount := 8
	worker := NewSimpleWorker(routinesCount)

	inspector, err := NewInspector()
	if err != nil {
		worker.AddError(err)
		return worker
	}

	stamp := time.Now().Unix()
	countTotal := int64(0)
	countOk := int64(0)
	countErrTotal := int64(0)
	countErrNotFound := int64(0)
	for i := 0; i < routinesCount; i++ {
		go func() {
			defer worker.Done()
			for nodeID := range nodeIDsChan {
				//log.Printf("INFO: KAD: fetching %s", nodeID)
				resp, err := inspector.LookupNode(context.Background(), &pb.LookupNodeRequest{
					Id: nodeID.String(),
				})

				if err != nil {
					atomic.AddInt64(&countErrTotal, 1)
					if st, ok := status.FromError(err); ok {
						if st.Message() == "node not found" {
							//log.Printf("WARN: KAD: skipping %s: not found", nodeID)
							atomic.AddInt64(&countErrNotFound, 1)
						} else {
							log.Printf("WARN: KAD: skipping %s: strange message: %s", nodeID, st.Message())
						}
					} else {
						log.Printf("WARN: KAD: skipping %s: strange reason: %s", nodeID, err)
					}
				} else {
					kadDataChan <- resp.GetNode()
					atomic.AddInt64(&countOk, 1)
				}

				step := int64(10)
				if atomic.AddInt64(&countTotal, 1)%step == 0 {
					log.Printf("INFO: KAD: total: %d, ok: %d, err(nf): %d(%d); %.2f rpm",
						countTotal, countOk, countErrTotal, countErrNotFound,
						float64(step)/float64(time.Now().Unix()-stamp)*60)
					stamp = time.Now().Unix()
				}
			}
		}()
	}
	return worker
}

func StartNeighborsKadDataFetcher(kadDataChan chan *pb.Node) Worker {
	worker := NewSimpleWorker(1)

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

func StartNodesSelfDataFetcher(kadDataChan chan *pb.Node, selfDataChan chan *pb.NodeInfoResponse) Worker {
	worker := NewSimpleWorker(1)

	inspector, err := NewInspector()
	if err != nil {
		worker.AddError(err)
		return worker
	}

	go func() {
		defer worker.Done()
		for node := range kadDataChan {
			info, err := inspector.NodeInfo(context.Background(), &pb.NodeInfoRequest{
				Id:      node.Id,
				Address: node.GetAddress(),
			})
			if err != nil {
				log.Printf("WARN: SELF: %s", err)
				continue
			}
			selfDataChan <- info
		}
	}()
	return worker
}

func StartOldKadDataLoader(db *pg.DB, nodeIDsChan chan storj.NodeID) Worker {
	worker := NewSimpleWorker(1)

	go func() {
		defer worker.Done()
		idsBytes := make([][]byte, 10)
		for {
			_, err := db.Query(&idsBytes, `
				WITH cte AS (SELECT id FROM storj3_nodes ORDER BY kad_checked_at ASC NULLS FIRST LIMIT 10)
				UPDATE storj3_nodes AS nodes SET kad_checked_at = NOW() FROM cte WHERE nodes.id = cte.id
				RETURNING nodes.id`)
			if err != nil {
				worker.AddError(err)
				return
			}
			ids, err := storj.NodeIDsFromBytes(idsBytes)
			if err != nil {
				worker.AddError(err)
				return
			}
			if len(ids) > 0 {
				log.Printf("INFO: DB-IDS: old %s - %s", ids[0], ids[len(ids)-1])
			} else {
				log.Print("INFO: DB-IDS: no old IDs")
				time.Sleep(10 * time.Second)
			}
			for _, id := range ids {
				nodeIDsChan <- id
			}
		}
	}()
	return worker
}

func StartOldSelfDataLoader(db *pg.DB, kadDataChan chan *pb.Node) Worker {
	worker := NewSimpleWorker(1)

	go func() {
		defer worker.Done()
		nodes := make([]*pb.Node, 10)
		for {
			_, err := db.Query(&nodes, "SELECT kad_params FROM storj3_nodes ORDER BY self_updated_at ASC NULLS FIRST LIMIT 10")
			if err != nil {
				worker.AddError(err)
				return
			}
			for _, node := range nodes {
				kadDataChan <- node
			}
		}
	}()
	return worker
}
