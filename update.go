package main

import (
	"context"
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

func StartNodesKadDataFetcher(nodeIDsChan chan storj.NodeID, kadDataChan chan *pb.Node, routinesCount int) Worker {
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
				resp, err := inspector.LookupNode(context.Background(), &pb.LookupNodeRequest{
					Id: nodeID.String(),
				})

				if err != nil {
					atomic.AddInt64(&countErrTotal, 1)
					if st, ok := status.FromError(err); ok {
						if st.Message() == "node not found" {
							atomic.AddInt64(&countErrNotFound, 1)
						} else {
							logWarn("KAD-FETCH", "skipping %s: strange message: %s", nodeID, st.Message())
						}
					} else {
						logWarn("KAD-FETCH", "skipping %s: strange reason: %s", nodeID, err)
					}
				} else {
					kadDataChan <- resp.GetNode()
					atomic.AddInt64(&countOk, 1)
				}

				if atomic.AddInt64(&countTotal, 1)%10 == 0 {
					logInfo("KAD-FETCH", "total: %d, ok: %d, err(nf): %d(%d); %.2f rpm",
						countTotal, countOk, countErrTotal, countErrNotFound,
						float64(countTotal)/float64(time.Now().Unix()-stamp)*60)
				}
			}
		}()
	}
	return worker
}

func StartNeighborsKadDataFetcher(kadDataChan chan *pb.Node, secondsInterval int) Worker {
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
				logWarn("NEI", "%s", err)
				continue
			}
			nodes := resp.GetNodes()
			logInfo("NEI", "got %d neighbor(s)", len(nodes))
			for _, node := range nodes {
				kadDataChan <- node
			}
			time.Sleep(time.Duration(secondsInterval) * time.Second)
		}
	}()
	return worker
}

func StartNodesSelfDataFetcher(nodesInChan chan *SelfUpdate_Kad, nodesOutChan chan *SelfUpdate_Self, routinesCount int) Worker {
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
			for node := range nodesInChan {
				info, err := inspector.NodeInfo(context.Background(), &pb.NodeInfoRequest{
					Id:      node.KadParams.Id,
					Address: node.KadParams.GetAddress(),
				})
				outNode := &SelfUpdate_Self{SelfUpdate_Kad: *node}

				if err != nil {
					if st, ok := status.FromError(err); ok {
						if !strings.HasSuffix(st.Message(), `connect: connection refused"`) &&
							!strings.HasSuffix(st.Message(), `connect: no route to host"`) &&
							!strings.HasSuffix(st.Message(), `transport error: context deadline exceeded"`) &&
							!strings.HasSuffix(st.Message(), `no such host"`) {
							logWarn("SELF-FETCH", "%s", err)
						}
					} else {
						logWarn("SELF-FETCH", "strange reason: %s", err)
					}
					outNode.SelfUpdateErr = err
					atomic.AddInt64(&countErrTotal, 1)
				} else {
					outNode.SelfParams = info
					atomic.AddInt64(&countOk, 1)
				}
				nodesOutChan <- outNode

				if atomic.AddInt64(&countTotal, 1)%10 == 0 {
					logInfo("SELF-FETCH", "total: %d, ok: %d, err: %d; %.2f rpm",
						countTotal, countOk, countErrTotal,
						float64(countTotal)/float64(time.Now().Unix()-stamp)*60)
				}
			}
		}()
	}
	return worker
}
