package main

import (
	"context"
	"log"
	"strings"
	"sync/atomic"
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

type UnmershableKadParams struct {
	ID        []byte
	KadParams struct {
		ID      string
		Address struct {
			Transport string
			Address   string
		}
		LastIP string
	}
}

type UnmershableKadParamsSlice []*UnmershableKadParams

func (p UnmershableKadParamsSlice) ToKadNodes() ([]*pb.Node, error) {
	nodes := make([]*pb.Node, len(p))
	var err error
	for i, params := range p {
		node := &pb.Node{Address: &pb.NodeAddress{}}
		node.Id, err = storj.NodeIDFromBytes(params.ID)
		if err != nil {
			return nil, merry.Errorf("wrong ID: %s: %s", params.ID, err)
		}
		if params.KadParams.Address.Transport == "" {
			params.KadParams.Address.Transport = "TCP_TLS_GRPC" //TODO: remove
		}
		transportID, ok := pb.NodeTransport_value[params.KadParams.Address.Transport]
		if !ok {
			return nil, merry.Errorf(`wrong transport name "%s" of node %s`,
				params.KadParams.Address.Transport, params.KadParams.ID)
		}
		node.Address.Transport = pb.NodeTransport(transportID)
		node.Address.Address = params.KadParams.Address.Address
		node.LastIp = params.KadParams.LastIP
		nodes[i] = node
	}
	return nodes, nil
}

func StartOldSelfDataLoader(db *pg.DB, kadDataChan chan *pb.Node) Worker {
	worker := NewSimpleWorker(1)

	go func() {
		defer worker.Done()
		nodesUn := make(UnmershableKadParamsSlice, 10)
		for {
			_, err := db.Query(&nodesUn, `
				WITH cte AS (SELECT id FROM storj3_nodes ORDER BY self_checked_at ASC NULLS FIRST LIMIT 10)
				UPDATE storj3_nodes AS nodes SET self_checked_at = NOW() FROM cte WHERE nodes.id = cte.id
				RETURNING nodes.id, nodes.kad_params`)
			if err != nil {
				worker.AddError(err)
				return
			}
			nodes, err := nodesUn.ToKadNodes()
			if err != nil {
				worker.AddError(err)
				return
			}
			if len(nodes) > 0 {
				log.Printf("INFO: DB-KAD: old %s - %s", nodes[0].Id, nodes[len(nodes)-1].Id)
			} else {
				log.Print("INFO: DB-KAD: no old KADs")
				time.Sleep(10 * time.Second)
			}
			for _, node := range nodes {
				kadDataChan <- node
			}
		}
	}()
	return worker
}

// func prettyPrint(unformatted proto.Message) string {
// 	m := jsonpb.Marshaler{Indent: "  ", EmitDefaults: true}
// 	formatted, err := m.MarshalToString(unformatted)
// 	if err != nil {
// 		fmt.Println("Error", err)
// 		os.Exit(1)
// 	}
// 	return formatted
// }
