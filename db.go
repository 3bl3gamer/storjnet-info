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

func StartNodesKadDataSaver(db *pg.DB, kadDataChan chan *KadDataExt, chunkSize int) Worker {
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
		err := saveChunked(db, chunkSize, kadDataChanI, func(tx *pg.Tx, items []interface{}) error {
			for _, node := range items {
				var xmax string
				_, err := db.QueryOne(&xmax, `
					INSERT INTO nodes (id, kad_params, location, kad_updated_at)
					VALUES (?, ?, ?, NOW())
					ON CONFLICT (id) DO UPDATE SET
						kad_params = EXCLUDED.kad_params, location = COALESCE(nodes.location, EXCLUDED.location), kad_updated_at = NOW()
					RETURNING xmax`, node.(*KadDataExt).Node.Id, node.(*KadDataExt).Node, node.(*KadDataExt).Location)
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

func StartNodesSelfDataSaver(db *pg.DB, selfDataChan chan *NodeInfoExt, chunkSize int) Worker {
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
		err := saveChunked(db, chunkSize, selfDataChanI, func(tx *pg.Tx, items []interface{}) error {
			for _, nodeI := range items {
				node := nodeI.(*NodeInfoExt)

				var xmax string
				_, err := db.QueryOne(&xmax, `
					INSERT INTO nodes (id, self_params, self_updated_at)
					VALUES (?, ?, NOW())
					ON CONFLICT (id) DO UPDATE SET self_params = EXCLUDED.self_params, self_updated_at = NOW()
					RETURNING xmax`, node.ID, node.Info)
				if err != nil {
					return merry.Wrap(err)
				}

				_, err = db.Exec(`
					INSERT INTO nodes_history (id, month_date, items)
					VALUES (?, date_trunc('month', now() at time zone 'utc')::date, ARRAY[(NOW(), ?, ?)::data_history_item])
					ON CONFLICT (id, month_date) DO UPDATE
					SET items = nodes_history.items || EXCLUDED.items
					`, node.ID, node.Info.Capacity.FreeDisk, node.Info.Capacity.FreeBandwidth)
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

func StartOldKadDataLoader(db *pg.DB, nodeIDsChan chan storj.NodeID, chunkSize int) Worker {
	worker := NewSimpleWorker(1)

	go func() {
		defer worker.Done()
		idsBytes := make([][]byte, chunkSize)
		for {
			_, err := db.Query(&idsBytes, `
				WITH cte AS (
					SELECT id FROM nodes
					WHERE kad_updated_at < NOW() - INTERVAL '15 minutes'
					ORDER BY kad_checked_at ASC NULLS FIRST LIMIT ?
				)
				UPDATE nodes SET kad_checked_at = NOW() FROM cte WHERE nodes.id = cte.id
				RETURNING nodes.id`, chunkSize)
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

func StartOldSelfDataLoader(db *pg.DB, kadDataChan chan *pb.Node, chunkSize int) Worker {
	worker := NewSimpleWorker(1)

	go func() {
		defer worker.Done()
		nodesStr := make([]string, chunkSize)
		nodes := make([]*pb.Node, chunkSize)
		for {
			_, err := db.Query(&nodesStr, `
				WITH cte AS (
					SELECT id FROM nodes
					WHERE kad_params IS NOT NULL AND self_updated_at < NOW() - INTERVAL '15 minutes'
					ORDER BY self_checked_at ASC NULLS FIRST LIMIT ?
				)
				UPDATE nodes SET self_checked_at = NOW() FROM cte WHERE nodes.id = cte.id
				RETURNING nodes.kad_params`, chunkSize)
			if err != nil {
				worker.AddError(err)
				return
			}

			nodes = nodes[:len(nodesStr)]
			for i, nodeStr := range nodesStr {
				node := &pb.Node{}
				if err := jsonpb.UnmarshalString(nodeStr, node); err != nil {
					worker.AddError(merry.Errorf("wrong node data: %s: %s", nodeStr, err))
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
		WITH active_nodes AS (
			SELECT * FROM nodes WHERE self_updated_at > NOW() - INTERVAL '24 hours'
		)
		INSERT INTO storjinfo.global_stats (
			count_total, count_hours,
			free_disk, free_disk_total, free_bandwidth,
			versions, types, countries, difficulties
		) VALUES ((
			SELECT count(*) FROM nodes
		), (
			SELECT array_agg((
				hours, (SELECT count(*) FROM nodes WHERE self_updated_at > NOW() - hours * INTERVAL '1 hour')
			)::activity_stat_item)
			FROM generate_series(1, 24) AS hours
		),
		(
			SELECT array_agg((
				perc, (
					SELECT percentile_cont(perc) WITHIN GROUP (ORDER BY (self_params->'capacity'->'free_disk')::bigint)
					FROM active_nodes WHERE self_params->'capacity'->'free_disk' IS NOT NULL
				)
			)::data_stat_item)
			FROM unnest(ARRAY[0.01, 0.05, 0.1, 0.25, 0.5, 0.75, 0.90, 0.95, 0.99]) AS perc
		), (
			SELECT array_agg((
				perc, (
					SELECT sum((self_params->'capacity'->'free_disk')::bigint)
					FROM active_nodes WHERE (self_params->'capacity'->'free_disk')::bigint <= (
						SELECT percentile_disc(perc) WITHIN GROUP (ORDER BY (self_params->'capacity'->'free_disk')::bigint)
						FROM active_nodes WHERE self_params->'capacity'->'free_disk' IS NOT NULL
					)
				)
			)::data_stat_item)
			FROM unnest(ARRAY[0.90, 0.95, 0.99, 0.995, 0.999, 1]) AS perc
		), (
			SELECT array_agg((
				perc, (
					SELECT percentile_cont(perc) WITHIN GROUP (ORDER BY (self_params->'capacity'->'free_bandwidth')::bigint)
					FROM active_nodes WHERE self_params->'capacity'->'free_bandwidth' IS NOT NULL
				)
			)::data_stat_item)
			FROM unnest(ARRAY[0.01, 0.05, 0.1, 0.25, 0.5, 0.75, 0.90, 0.95, 0.99]) AS perc
		),
		(
			SELECT array_agg((version, cnt)::version_stat_item ORDER BY version) FROM (
				SELECT self_params->'version'->>'version' AS version, count(*) AS cnt FROM active_nodes GROUP BY version
			) AS t
		), (
			SELECT array_agg((type, cnt)::type_stat_item ORDER BY type) FROM (
				SELECT self_params->>'type' AS type, count(*) AS cnt FROM active_nodes GROUP BY type
			) AS t
		), (
			SELECT array_agg((country, cnt)::country_stat_item ORDER BY cnt) FROM (
				SELECT (location).country, count(*) AS cnt FROM active_nodes GROUP BY (location).country
			) AS t
		), (
			SELECT array_agg((dif, count)::difficulty_stat_item ORDER BY dif) FROM (
				SELECT length(substring(('x'||encode(id, 'hex'))::bit(256)::text FROM '0*$')) AS dif, count(*) FROM active_nodes GROUP BY dif
			) AS t)
		)
		`)
	/*
		SELECT array_agg((perc, cnt)::data_stat_item) FROM (
			SELECT perc, percentile_cont(perc) WITHIN GROUP (ORDER BY (self_params->'capacity'->>'free_disk')::bigint) as cnt
			FROM nodes, unnest(ARRAY[0.01, 0.05, 0.1, 0.25, 0.5, 0.75, 0.90, 0.95, 0.99]) AS perc
			WHERE self_params->'capacity'->>'free_disk' IS NOT NULL
			GROUP BY perc
		) AS t;
	*/
	return merry.Wrap(err)
}

/*
func SaveNodesDataUsageHistory(db *pg.DB) error {
	_, err := db.Exec(`
		INSERT INTO nodes_history (id, month_date, items)
			SELECT
				id, date_trunc('month', now() at time zone 'utc')::date,
				ARRAY[(NOW(), self_params->'capacity'->>'free_disk', self_params->'capacity'->>'free_bandwidth')::data_history_item]
			FROM nodes
			WHERE self_updated_at > NOW() - INTERVAL '3 days'
				AND self_params ? 'capacity'
		ON CONFLICT (id, month_date) DO UPDATE
		SET items = nodes_history.items || EXCLUDED.items
		`)
	return merry.Wrap(err)
}
*/
