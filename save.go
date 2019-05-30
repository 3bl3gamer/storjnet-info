package main

import (
	"log"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg"
	"storj.io/storj/pkg/pb"
)

func StartNodesKadDataSaver(db *pg.DB, kadDataChan chan *pb.Node) Worker {
	worker := NewSimpleWorker()
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
				INSERT INTO storj3_nodes (id, kad_params, kad_updated_at)
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
			log.Printf("imported %d kad nodes, %d new", count, countNew)
			return nil
		})
		log.Printf("done, imported %d kad nodes, %d new", count, countNew)
		if err != nil {
			worker.AddError(err)
		}
	}()

	return worker
}
