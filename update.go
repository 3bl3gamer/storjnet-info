package main

import (
	"context"
	"log"
	"strings"
	"sync/atomic"
	"time"

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

				if atomic.AddInt64(&countTotal, 1)%10 == 0 {
					log.Printf("INFO: KAD: total: %d, ok: %d, err(nf): %d(%d); %.2f rpm",
						countTotal, countOk, countErrTotal, countErrNotFound,
						float64(countTotal)/float64(time.Now().Unix()-stamp)*60)
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

func StartNodesSelfDataFetcher(kadDataChan chan *pb.Node, selfDataChan chan *NodeInfoWithID) Worker {
	routinesCount := 16
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
	for i := 0; i < routinesCount; i++ {
		go func() {
			defer worker.Done()
			for node := range kadDataChan {
				info, err := inspector.NodeInfo(context.Background(), &pb.NodeInfoRequest{
					Id:      node.Id,
					Address: node.GetAddress(),
				})

				if err != nil {
					if st, ok := status.FromError(err); ok {
						if !strings.HasSuffix(st.Message(), "connect: connection refused") &&
							!strings.HasSuffix(st.Message(), "transport error: context deadline exceeded") {
							log.Printf("WARN: SELF: %s", err)
						}
					} else {
						log.Printf("WARN: SELF: strange reason: %s", err)
					}
					atomic.AddInt64(&countErrTotal, 1)
				} else {
					selfDataChan <- &NodeInfoWithID{node.Id, info}
					atomic.AddInt64(&countOk, 1)
				}

				if atomic.AddInt64(&countTotal, 1)%10 == 0 {
					log.Printf("INFO: SELF: total: %d, ok: %d, err: %d; %.2f rpm",
						countTotal, countOk, countErrTotal,
						float64(countTotal)/float64(time.Now().Unix()-stamp)*60)
				}
			}
		}()
	}
	return worker
}
