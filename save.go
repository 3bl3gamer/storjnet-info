package main

import (
	"log"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type NodeInfoWithID struct {
	ID   storj.NodeID
	Info *pb.NodeInfoResponse
}

func StartNodesKadDataSaver(db *pg.DB, kadDataChan chan *pb.Node) Worker {
	worker := NewSimpleWorker(1)
	kadDataChanI := make(chan interface{}, 16)

	go func() {
		for node := range kadDataChan {
			kadDataChanI <- node
		}
		close(kadDataChanI)
	}()

	count := 0
	countNew := 0
	go func() {
		defer worker.Done()
		err := saveChunked(db, 10, kadDataChanI, func(tx *pg.Tx, items []interface{}) error {
			for _, node := range items {
				var xmax string
				_, err := db.QueryOne(&xmax, `
					INSERT INTO nodes (id, kad_params, kad_updated_at)
					VALUES (?, ?, NOW())
					ON CONFLICT (id) DO UPDATE SET kad_params = EXCLUDED.kad_params, kad_updated_at = NOW()
					RETURNING xmax`, node.(*pb.Node).Id, node)
				if err != nil {
					return merry.Wrap(err)
				}
				count++
				if xmax == "0" {
					countNew++
				}
			}
			log.Printf("INFO: SAVE-KAD: imported %d kad nodes, %d new", count, countNew)
			return nil
		})
		log.Printf("INFO: SAVE-KAD: done, imported %d kad nodes, %d new", count, countNew)
		if err != nil {
			worker.AddError(err)
		}
	}()

	return worker
}

func StartNodesSelfDataSaver(db *pg.DB, selfDataChan chan *NodeInfoWithID) Worker {
	worker := NewSimpleWorker(1)
	selfDataChanI := make(chan interface{}, 16)

	go func() {
		for node := range selfDataChan {
			selfDataChanI <- node
		}
		close(selfDataChanI)
	}()

	count := 0
	countNew := 0
	go func() {
		defer worker.Done()
		err := saveChunked(db, 10, selfDataChanI, func(tx *pg.Tx, items []interface{}) error {
			for _, node := range items {
				var xmax string
				_, err := db.QueryOne(&xmax, `
					INSERT INTO nodes (id, self_params, self_updated_at)
					VALUES (?, ?, NOW())
					ON CONFLICT (id) DO UPDATE SET self_params = EXCLUDED.self_params, self_updated_at = NOW()
					RETURNING xmax`, node.(*NodeInfoWithID).ID, node.(*NodeInfoWithID).Info)
				if err != nil {
					return merry.Wrap(err)
				}
				count++
				if xmax == "0" {
					countNew++
				}
			}
			log.Printf("INFO: SAVE-SELF: imported %d self nodes data, %d new", count, countNew)
			return nil
		})
		log.Printf("INFO: SAVE-SELF: done, imported %d self nodes data, %d new", count, countNew)
		if err != nil {
			worker.AddError(err)
		}
	}()

	return worker
}
