package nodes

import (
	"storjnet/utils"

	"github.com/ansel1/merry"
)

func SaveStats() error {
	db := utils.MakePGConnection()

	_, err := db.Exec(`
		INSERT INTO storjnet.node_stats (
			count_total,
			active_count_hours,
			active_count_proto,
			all_sat_offers_count_hours,
			per_sat_offers_count_hours,
			countries,
			ports
		) VALUES ((
			SELECT count(*) FROM nodes
		), (
			SELECT jsonb_object_agg(
				hours, (SELECT count(*) FROM nodes WHERE updated_at > NOW() - hours::float * INTERVAL '1 hour')
				ORDER BY hours
			)
			FROM (SELECT generate_series(1, 24) AS hours UNION SELECT unnest(ARRAY[48, 72, 0.5])) t
		), (
			SELECT jsonb_build_object(
				'tcp', (SELECT count(*) FROM nodes WHERE tcp_updated_at > NOW() - INTERVAL '1 day'),
				'quic', (SELECT count(*) FROM nodes WHERE quic_updated_at > NOW() - INTERVAL '1 day')
			)
		), (
			SELECT jsonb_object_agg(
				hours, (
					SELECT count(DISTINCT node_id) FROM nodes_sat_offers
					WHERE stamps[array_upper(stamps, 1)] > NOW() - hours::int * INTERVAL '1 hour'
				)
				ORDER BY hours
			)
			FROM unnest(ARRAY[1, 3, 6, 12, 24, 48, 72]) AS hours
		), (
			SELECT jsonb_object_agg(
				cur_sat_name, (
					SELECT jsonb_object_agg(
						hours, (
							SELECT count(*) FROM nodes_sat_offers
							WHERE satellite_name = cur_sat_name
							  AND stamps[array_upper(stamps, 1)] > NOW() - hours::int * INTERVAL '1 hour'
						)
						ORDER BY hours
					)
					FROM unnest(ARRAY[1, 3, 6, 12, 24, 48, 72]) AS hours
				)
				ORDER BY cur_sat_name
			)
			FROM (
				SELECT DISTINCT satellite_name AS cur_sat_name
				FROM nodes_sat_offers
				WHERE stamps[array_upper(stamps, 1)] > NOW() - INTERVAL '1 day'
			) AS t
		), (
			SELECT jsonb_object_agg(country, cnt) FROM (
				SELECT COALESCE(location->>'country', '<unknown>') AS country, count(*) AS cnt
				FROM nodes
				WHERE updated_at > NOW() - INTERVAL '1 day'
				GROUP BY country
			) AS t
		), (
			SELECT jsonb_object_agg(port, cnt) FROM (
				SELECT port, count(*) AS cnt
				FROM nodes
				WHERE updated_at > NOW() - INTERVAL '1 day'
				GROUP BY port
				ORDER BY cnt DESC
				LIMIT 100
			) AS t
		))`)
	if err != nil {
		return merry.Wrap(err)
	}

	dailyStats := func(kind, activeNodesSQL string, params ...interface{}) error {
		params = append(params, kind, kind, kind)
		_, err := db.Exec(`
			WITH dates (today, yesterday) AS (VALUES(
				(NOW() at time zone 'UTC')::date,
				(NOW() at time zone 'UTC')::date - INTERVAL '1 day'
			)), active_nodes AS (
				`+activeNodesSQL+`
			)
			INSERT INTO node_daily_stats (date, kind, node_ids, come_node_ids, left_node_ids)
			VALUES (
				(SELECT today FROM dates),
				?,
				(SELECT array_agg(id ORDER BY id) FROM active_nodes),
				(
					SELECT COALESCE(array_agg(id ORDER BY id), '{}'::bytea[])
					FROM active_nodes
					LEFT OUTER JOIN (SELECT unnest(node_ids) AS yest_id FROM node_daily_stats, dates WHERE date = yesterday AND kind = ?) AS t
					ON (id = yest_id) WHERE yest_id IS NULL
				), (
					SELECT COALESCE(array_agg(yest_id ORDER BY yest_id), '{}'::bytea[])
					FROM unnest((SELECT node_ids FROM node_daily_stats, dates WHERE date = yesterday AND kind = ?)) as yest_id
					WHERE NOT EXISTS (SELECT 1 FROM active_nodes WHERE id = yest_id)
				)
			)
			ON CONFLICT (date, kind) DO UPDATE SET
				node_ids = EXCLUDED.node_ids,
				come_node_ids = EXCLUDED.come_node_ids,
				left_node_ids = EXCLUDED.left_node_ids`,
			params...)
		return merry.Wrap(err)
	}

	err = dailyStats("active", `
		SELECT id FROM nodes WHERE updated_at > NOW() - INTERVAL '24 hours'`)
	if err != nil {
		return merry.Wrap(err)
	}

	err = dailyStats("offered_by_sats", `
		SELECT DISTINCT node_id AS id FROM nodes_sat_offers
		WHERE stamps[array_upper(stamps, 1)] > NOW() - INTERVAL '24 hours'`)
	if err != nil {
		return merry.Wrap(err)
	}

	var satNames []string
	_, err = db.Query(&satNames, `
		SELECT DISTINCT satellite_name FROM nodes_sat_offers
		WHERE stamps[array_upper(stamps, 1)] > NOW() - INTERVAL '24 hours'`)
	if err != nil {
		return merry.Wrap(err)
	}
	for _, satName := range satNames {
		err = dailyStats("offered_by_sat:"+satName, `
			SELECT node_id AS id FROM nodes_sat_offers
			WHERE satellite_name = ?
			  AND stamps[array_upper(stamps, 1)] > NOW() - INTERVAL '24 hours'`,
			satName)
		if err != nil {
			return merry.Wrap(err)
		}
	}
	return nil
}
