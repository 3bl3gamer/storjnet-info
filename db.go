package main

import (
	"database/sql"
	"hash/fnv"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg"
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
				_, err := tx.QueryOne(&xmax, `
					INSERT INTO nodes (id, kad_params, location, kad_updated_at, kad_checked_at)
					VALUES (?, ?, ?, NOW(), NOW())
					ON CONFLICT (id) DO UPDATE SET
						kad_params = EXCLUDED.kad_params,
						location = COALESCE(nodes.location, EXCLUDED.location),
						kad_updated_at = NOW(),
						kad_checked_at = GREATEST(nodes.kad_checked_at, EXCLUDED.kad_checked_at)
					RETURNING xmax`, node.(*KadDataExt).Node.Id, node.(*KadDataExt).Node, node.(*KadDataExt).Location)
				if err != nil {
					return merry.Wrap(err)
				}
				count++
				if xmax == "0" {
					countNew++
				}
			}
			logInfo("SAVE-KAD", "imported %d kad nodes, %d new", count, countNew)
			return nil
		})
		logInfo("SAVE-KAD", "done, imported %d kad nodes, %d new", count, countNew)
		if err != nil {
			worker.AddError(err)
		}
	}()

	return worker
}

func StartNodesSelfDataSaver(db *pg.DB, selfDataChan chan *SelfUpdate_Self, chunkSize int) Worker {
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
				node := nodeI.(*SelfUpdate_Self)
				var xmax string

				var lastActivityStamp time.Time
				var lastFreeDataStamp time.Time
				_, err := tx.QueryOne(pg.Scan(&lastActivityStamp, &lastFreeDataStamp), `
					SELECT
						to_timestamp(activity_stamps[array_length(activity_stamps, 1)]),
						(free_data_items[array_length(free_data_items, 1)]).stamp
					FROM nodes_history
					WHERE id = ? AND month_date = date_trunc('month', now() at time zone 'utc')::date
					`, node.ID)
				if err != nil && err != pg.ErrNoRows {
					return merry.Wrap(err)
				}

				if node.SelfUpdateErr == nil {
					_, err := tx.QueryOne(&xmax, `
						INSERT INTO nodes (id, self_params, self_updated_at)
						VALUES (?, ?, NOW())
						ON CONFLICT (id) DO UPDATE SET self_params = EXCLUDED.self_params, self_updated_at = NOW()
						RETURNING xmax`, node.ID, node.SelfParams)
					if err != nil {
						return merry.Wrap(err)
					}

					if time.Now().Sub(lastFreeDataStamp) >= 15*time.Minute {
						_, err = tx.Exec(`
						INSERT INTO nodes_history (id, month_date, free_data_items)
						VALUES (?, date_trunc('month', now() at time zone 'utc')::date, ARRAY[(NOW(), ?, ?)::data_history_item])
						ON CONFLICT (id, month_date) DO UPDATE
						SET free_data_items = nodes_history.free_data_items || EXCLUDED.free_data_items
						`, node.ID, node.SelfParams.Capacity.FreeDisk, node.SelfParams.Capacity.FreeBandwidth)
						if err != nil {
							return merry.Wrap(err)
						}
					}
				}

				if time.Now().Sub(lastActivityStamp) >= 5*time.Minute {
					var lastErr sql.NullString
					if node.SelfUpdateErr != nil {
						lastErr = sql.NullString{node.SelfUpdateErr.Error(), true}
					}
					_, err := tx.Exec(`
						INSERT INTO nodes_history (id, month_date, activity_stamps, last_self_params_error)
						VALUES (?, date_trunc('month', now() at time zone 'utc')::date, ARRAY[(EXTRACT(EPOCH FROM NOW())/10)::int*10 + ?::int], ?)
						ON CONFLICT (id, month_date) DO UPDATE
						SET activity_stamps = nodes_history.activity_stamps || EXCLUDED.activity_stamps,
							last_self_params_error = COALESCE(EXCLUDED.last_self_params_error, nodes_history.last_self_params_error)
						`, node.ID, node.SelfUpdateErr != nil, lastErr)
					if err != nil {
						return merry.Wrap(err)
					}
				}

				count++
				if xmax == "0" {
					countNew++
				}
			}
			logInfo("SAVE-SELF", "imported %d self nodes data, %d new", count, countNew)
			return nil
		})
		logInfo("SAVE-SELF", "done, imported %d self nodes data, %d new", count, countNew)
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
		for {
			idsBytes := make([][]byte, chunkSize)
			_, err := db.Query(&idsBytes, `
				WITH cte AS (
					SELECT id FROM nodes
					WHERE kad_updated_at IS NULL OR kad_updated_at < NOW() - INTERVAL '15 minutes'
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
				logInfo("DB-IDS", "old %s - %s (%d)", ids[0], ids[len(ids)-1], len(ids))
			} else {
				logInfo("DB-IDS", "no old IDs")
				time.Sleep(10 * time.Second)
			}
			for _, id := range ids {
				nodeIDsChan <- id
			}
		}
	}()
	return worker
}

func StartOldSelfDataLoader(db *pg.DB, kadDataChan chan *SelfUpdate_Kad, chunkSize int) Worker {
	worker := NewSimpleWorker(1)

	go func() {
		defer worker.Done()
		for {
			nodes := make([]*SelfUpdate_Kad, chunkSize)
			_, err := db.Query(&nodes, `
				WITH cte AS (
					SELECT id FROM nodes
					WHERE kad_params IS NOT NULL AND (self_updated_at IS NULL OR self_updated_at < NOW() - INTERVAL '5 minutes')
					ORDER BY self_checked_at ASC NULLS FIRST LIMIT ?
				)
				UPDATE nodes SET self_checked_at = NOW() FROM cte WHERE nodes.id = cte.id
				RETURNING nodes.kad_params`, chunkSize)
			if err != nil {
				worker.AddError(err)
				return
			}

			if len(nodes) > 0 {
				logInfo("DB-KAD", "old %s - %s (%d)", nodes[0].KadParams.Id, nodes[len(nodes)-1].KadParams.Id, len(nodes))
			} else {
				logInfo("DB-KAD", "no old KADs")
				time.Sleep(10 * time.Second)
			}

			for _, node := range nodes {
				node.ID = NodeIDExt(node.KadParams.Id)
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
	return merry.Wrap(err)
}

func SaveVisit(db *pg.DB, ipAddress, userAgent, urlPath string) error {
	hashCalc := fnv.New64a()
	if _, err := hashCalc.Write([]byte(ipAddress)); err != nil {
		return merry.Wrap(err)
	}
	visitorHash := hashCalc.Sum(nil)
	if _, err := hashCalc.Write([]byte(userAgent + "_" + urlPath)); err != nil {
		return merry.Wrap(err)
	}
	hash := hashCalc.Sum(nil)
	_, err := db.Exec(`
		INSERT INTO visits (visit_hash, day_date, visitor_hash, user_agent, path, count)
		VALUES (?, (now() at time zone 'utc')::date, ?, ?, ?, 1)
		ON CONFLICT (visit_hash, day_date) DO UPDATE SET count = visits.count + 1
		`, hash, visitorHash, userAgent, urlPath)
	return merry.Wrap(err)
}
