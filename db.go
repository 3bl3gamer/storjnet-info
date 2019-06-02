package main

import (
	"log"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg"
	"github.com/gogo/protobuf/jsonpb"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

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

func StartOldKadDataLoader(db *pg.DB, nodeIDsChan chan storj.NodeID) Worker {
	worker := NewSimpleWorker(1)

	go func() {
		defer worker.Done()
		idsBytes := make([][]byte, 10)
		for {
			_, err := db.Query(&idsBytes, `
				WITH cte AS (SELECT id FROM nodes ORDER BY kad_checked_at ASC NULLS FIRST LIMIT 10)
				UPDATE nodes AS nodes SET kad_checked_at = NOW() FROM cte WHERE nodes.id = cte.id
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
		nodesStr := make([]string, 10)
		nodes := make([]*pb.Node, 10)
		for {
			_, err := db.Query(&nodesStr, `
				WITH cte AS (SELECT id FROM nodes WHERE kad_params IS NOT NULL ORDER BY self_checked_at ASC NULLS FIRST LIMIT 10)
				UPDATE nodes AS nodes SET self_checked_at = NOW() FROM cte WHERE nodes.id = cte.id
				RETURNING nodes.kad_params`)
			if err != nil {
				worker.AddError(err)
				return
			}

			nodes = nodes[:len(nodesStr)]
			for i, nodeStr := range nodesStr {
				node := &pb.Node{}
				if err := jsonpb.UnmarshalString(nodeStr, node); err != nil {
					worker.AddError(err)
					return
				}
				nodes[i] = node
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

func SaveGlobalNodesStats(db *pg.DB) error {
	_, err := db.Exec(`
		INSERT INTO storjinfo.global_stats (count_total, count_active_24h, count_active_12h, versions) VALUES (
			(SELECT count(*) FROM nodes),
			(SELECT count(*) FROM nodes WHERE self_updated_at > NOW() - INTERVAL '24 hours'),
			(SELECT count(*) FROM nodes WHERE self_updated_at > NOW() - INTERVAL '12 hours'),
			(SELECT jsonb_object_agg(COALESCE(version, 'null'), cnt) FROM (
				WITH cte AS (SELECT self_params->'version'->>'version' AS version FROM nodes)
				SELECT version, count(*) AS cnt FROM cte GROUP BY version
			) AS t)
		)
		`)
	return merry.Wrap(err)
}
