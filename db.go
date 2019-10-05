package main

import (
	"database/sql"
	"hash/fnv"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
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
			ids := make([]storj.NodeID, len(items))
			for i, nodeI := range items {
				ids[i] = nodeI.(*KadDataExt).Node.Id
			}
			if _, err := tx.Exec("SELECT 1 FROM nodes WHERE id IN (?) FOR UPDATE", pg.In(ids)); err != nil {
				return merry.Wrap(err)
			}

			for _, nodeI := range items {
				node := nodeI.(*KadDataExt)
				ip := sql.NullString{Valid: false} //go-pg пытается вставить айпи как строку и иногда пытется вставить "<nil>"
				if node.IPAddress != nil {
					ip = sql.NullString{String: node.IPAddress.String(), Valid: true}
				}
				var xmax string
				_, err := tx.QueryOne(&xmax, `
					INSERT INTO nodes (id, kad_params, last_ip, location, kad_updated_at, kad_checked_at)
					VALUES (?, ?, ?, ?, NOW(), NOW())
					ON CONFLICT (id) DO UPDATE SET
						kad_params = EXCLUDED.kad_params,
						last_ip = COALESCE(EXCLUDED.last_ip, nodes.last_ip),
						location = COALESCE(EXCLUDED.location, nodes.location),
						kad_updated_at = NOW(),
						kad_checked_at = GREATEST(nodes.kad_checked_at, EXCLUDED.kad_checked_at)
					RETURNING xmax`, node.Node.Id, node.Node, ip, node.Location)
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
			ids := make([]NodeIDExt, len(items))
			for i, nodeI := range items {
				ids[i] = nodeI.(*SelfUpdate_Self).ID
			}
			if _, err := tx.Exec("SELECT 1 FROM nodes WHERE id IN (?) FOR UPDATE", pg.In(ids)); err != nil {
				return merry.Wrap(err)
			}

			for _, nodeI := range items {
				node := nodeI.(*SelfUpdate_Self)
				var xmax string

				var lastActivityStamp time.Time
				var prevFreeDataStamp time.Time
				var lastFreeData DataHistoryItem
				_, err := tx.QueryOne(pg.Scan(&lastActivityStamp, &prevFreeDataStamp, &lastFreeData), `
					SELECT
						to_timestamp(activity_stamps[array_length(activity_stamps, 1)]),
						(free_data_items[array_length(free_data_items, 1)-1]).Stamp,
						(free_data_items[array_length(free_data_items, 1)])
					FROM nodes_history
					WHERE id = ? AND date = (now() at time zone 'utc')::date
					`, node.ID)
				if err != nil && err != pg.ErrNoRows {
					return merry.Wrap(err)
				}

				if node.SelfUpdateErr == nil {
					_, err := tx.QueryOne(&xmax, `
						INSERT INTO nodes (id, self_params, self_updated_at)
						VALUES (?, ?, NOW())
						ON CONFLICT (id) DO UPDATE SET
							self_params = CASE
								WHEN ?
								THEN jsonb_set(COALESCE(nodes.self_params, '{"version":null}'), '{version}', ?)
								ELSE COALESCE(nodes.self_params, EXCLUDED.self_params)
							END,
							self_updated_at = NOW()
						RETURNING xmax`,
						node.ID, node.SelfParams, node.AccessIsDenied, `{"version": "`+node.VersionHint+`"}`)
					if err != nil {
						return merry.Wrap(err)
					}

					if !node.AccessIsDenied {
						capacity := node.SelfParams.Capacity
						diskChanged := lastFreeData.FreeDisk != capacity.FreeDisk
						bandChanged := lastFreeData.FreeBandwidth != capacity.FreeBandwidth
						prevTimedelta := time.Now().Sub(prevFreeDataStamp)
						lastTimedelta := time.Now().Sub(lastFreeData.Stamp)
						// остаток места и трафика кешируются нодой на час;
						// если за два часа значение не поменялось, скорее всеготрафика всё-таки не было, и значение нужно сохранить
						if diskChanged || bandChanged || lastTimedelta > 2*time.Hour {
							var conflictAction string
							if lastTimedelta < 14*time.Minute && prevTimedelta < 20*time.Minute {
								conflictAction = "SET free_data_items[array_length(nodes_history.free_data_items, 1)] = EXCLUDED.free_data_items[1]"
							} else {
								conflictAction = "SET free_data_items = nodes_history.free_data_items || EXCLUDED.free_data_items"
							}
							_, err = tx.Exec(`
								INSERT INTO nodes_history (id, date, free_data_items)
								VALUES (?, (now() at time zone 'utc')::date, ARRAY[(NOW(), ?, ?)::data_history_item])
								ON CONFLICT (id, date) DO UPDATE
								`+conflictAction, node.ID, capacity.FreeDisk, capacity.FreeBandwidth)
							if err != nil {
								return merry.Wrap(err)
							}
						}
					}
				}

				if time.Now().Sub(lastActivityStamp) >= 4*time.Minute {
					var lastErr sql.NullString
					if node.SelfUpdateErr != nil {
						lastErr = sql.NullString{node.SelfUpdateErr.Error(), true}
					}
					_, err := tx.Exec(`
						INSERT INTO nodes_history (id, date, activity_stamps, last_self_params_error)
						VALUES (?, (now() at time zone 'utc')::date, ARRAY[(EXTRACT(EPOCH FROM NOW())/10)::int*10 + ?::int], ?)
						ON CONFLICT (id, date) DO UPDATE
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
			var ids storj.NodeIDList
			idsBytes := make([][]byte, chunkSize)
			err := db.RunInTransaction(func(tx *pg.Tx) error {
				_, err := tx.Query(&idsBytes, `
					SELECT id FROM nodes
					WHERE (kad_updated_at IS NULL OR kad_updated_at < NOW() - INTERVAL '15 minutes')
					  AND (GREATEST(kad_updated_at, self_updated_at, created_at) > NOW() - INTERVAL '3 days')
					ORDER BY kad_checked_at ASC NULLS FIRST
					LIMIT ?
					FOR UPDATE`, chunkSize)
				if err != nil {
					return merry.Wrap(err)
				}
				if len(idsBytes) == 0 {
					return nil
				}
				ids, err = storj.NodeIDsFromBytes(idsBytes)
				if err != nil {
					return merry.Wrap(err)
				}
				_, err = tx.Exec(`UPDATE nodes SET kad_checked_at = NOW() WHERE id IN (?)`, pg.In(ids))
				return merry.Wrap(err)
			})
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
			err := db.RunInTransaction(func(tx *pg.Tx) error {
				_, err := tx.Query(&nodes, `
					SELECT id, kad_params FROM nodes
					WHERE (self_updated_at IS NULL OR self_updated_at < NOW() - INTERVAL '5 minutes')
					  AND (GREATEST(kad_updated_at, self_updated_at, created_at) > NOW() - INTERVAL '3 days')
					ORDER BY self_checked_at ASC NULLS FIRST
					LIMIT ?
					FOR UPDATE`, chunkSize)
				if err != nil {
					return merry.Wrap(err)
				}
				if len(nodes) == 0 {
					return nil
				}
				idsBytes := make([]NodeIDExt, len(nodes))
				for i, node := range nodes {
					idsBytes[i] = node.ID
				}
				_, err = tx.Exec(`UPDATE nodes SET self_checked_at = NOW() WHERE id IN (?)`, pg.In(idsBytes))
				return merry.Wrap(err)
			})

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
				node.ID = NodeIDExt{node.KadParams.Id}
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
			SELECT array_agg((version, cnt)::version_stat_item ORDER BY string_to_array(substr(version, 2), '.')::int[]) FROM (
				SELECT self_params->'version'->>'version' AS version, count(*) AS cnt FROM active_nodes GROUP BY version
			) AS t
		), (
			SELECT array_agg((type, cnt)::type_stat_item ORDER BY type) FROM (
				SELECT COALESCE((self_params->'type')::int, 0) AS type, count(*) AS cnt FROM active_nodes GROUP BY type
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

type GlobalNodesHistoryData struct {
	StartTime     time.Time          `json:"startTime"`
	EndTime       time.Time          `json:"endTime"`
	Stamps        []int64            `json:"stamps"`
	CountHours    map[int64][]int64  `json:"countHours"`
	CountVersions map[string][]int64 `json:"countVersions"`
}

func LoadGlobalNodesHistoryData(db *pg.DB, daysBack int) (*GlobalNodesHistoryData, error) {
	endTime := time.Date(2019, 10, 6, 0, 0, 0, 0, time.UTC) //time.Now().In(time.UTC)
	startTime := endTime.AddDate(0, -1, 0)
	if daysBack > 0 {
		startTime = endTime.AddDate(0, 0, -daysBack)
	}

	var globalStats []*GlobalStat
	err := db.Model(&globalStats).Where("created_at >= ? AND created_at < ?", startTime, endTime).Order("id").Select()
	if err != nil {
		return nil, merry.Wrap(err)
	}
	l := len(globalStats)

	stamps := make([]int64, l)
	for i, stat := range globalStats {
		stamps[i] = stat.CreatedAt.Unix()
	}

	countHours := map[int64][]int64{24: make([]int64, l), 12: make([]int64, l), 3: make([]int64, l)}
	for i, stat := range globalStats {
		for _, item := range stat.CountHours {
			if counts, ok := countHours[item.Hours]; ok {
				counts[i] = item.Count
			}
		}
	}

	countVersions := make(map[string][]int64)
	maxVersionsCounts := make(map[string]int64)
	maxVersionCount := int64(0)
	for _, stat := range globalStats {
		for _, item := range stat.Versions {
			if item.Count > maxVersionsCounts[item.Version] {
				maxVersionsCounts[item.Version] = item.Count
			}
			if item.Count > maxVersionCount {
				maxVersionCount = item.Count
			}
		}
	}
	for version, count := range maxVersionsCounts {
		if count < maxVersionCount/100 {
			delete(maxVersionsCounts, version)
		} else {
			countVersions[version] = make([]int64, l)
		}
	}
	for i, stat := range globalStats {
		for _, item := range stat.Versions {
			if _, ok := maxVersionsCounts[item.Version]; ok {
				countVersions[item.Version][i] = item.Count
			}
		}
	}

	return &GlobalNodesHistoryData{
		StartTime:     startTime,
		EndTime:       endTime,
		Stamps:        stamps,
		CountHours:    countHours,
		CountVersions: countVersions,
	}, nil
}
