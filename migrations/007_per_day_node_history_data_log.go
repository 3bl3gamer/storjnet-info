package main

import (
	"errors"

	"github.com/go-pg/migrations"
)

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			-- 112

			INSERT INTO storjinfo.nodes_history (id, month_date, free_data_items) (
				SELECT id, month_date, array_agg(item ORDER BY (item).stamp) AS free_data_items FROM (
					SELECT DISTINCT ON (id, (item).free_disk, (item).free_bandwidth, hstamp) id, date_trunc('month', (item).stamp AT TIME ZONE 'UTC')::date as month_date, item FROM (
						SELECT id, item,
							(
								CASE WHEN
									(item).stamp - lag((item).stamp, 1, (item).stamp) OVER (PARTITION BY id, month_date ORDER BY (item).stamp) < INTERVAL '30 minutes'
									AND lead((item).stamp, 1, (item).stamp) OVER (PARTITION BY id, month_date ORDER BY (item).stamp) - (item).stamp < INTERVAL '30 minutes'
								THEN
									'2000-01-01'::timestamptz
								ELSE
									(item).stamp
								END
							) AS hstamp
						FROM (
							SELECT id, month_date, unnest(free_data_items) AS item FROM nodes_history
						) AS t
					) AS t
					ORDER BY id, (item).free_disk, (item).free_bandwidth, hstamp, (item).stamp
				) AS t
				GROUP BY (id, month_date)
			)
			ON CONFLICT (id, month_date) DO UPDATE SET free_data_items = EXCLUDED.free_data_items;

			-- 87

			INSERT INTO storjinfo.nodes_history (id, month_date, free_data_items) (
				SELECT id, month_date, array_agg(item ORDER BY (item).stamp) FROM (
					SELECT id, month_date,
						(
							CASE WHEN (next - prev) < INTERVAL '2 hours' THEN
								((item).stamp + ((next-(item).stamp)/2 + (prev-(item).stamp)/2)*2/3, (item).free_disk, (item).free_bandwidth)::data_history_item
							ELSE
								((item).stamp, (item).free_disk, (item).free_bandwidth)::data_history_item
							END
						) AS item
						FROM (
						SELECT id, month_date, item,
							lag((item).stamp, 1, (item).stamp - INTERVAL '60 minutes') OVER (PARTITION BY id, month_date ORDER BY (item).stamp) AS prev,
							lead((item).stamp, 1, (item).stamp + INTERVAL '60 minutes') OVER (PARTITION BY id, month_date ORDER BY (item).stamp) AS next
						FROM (
							SELECT id, month_date, unnest(free_data_items) AS item FROM nodes_history
						) AS t
					) AS t
				) AS t
				GROUP BY (id, month_date)
			)
			ON CONFLICT (id, month_date) DO UPDATE SET free_data_items = EXCLUDED.free_data_items;

			-- 87

			ALTER TABLE storjinfo.nodes_history RENAME TO nodes_history_old;

			CREATE TABLE storjinfo.nodes_history (
				id bytea NOT NULL,
				date date NOT NULL,
				free_data_items data_history_item[] NOT NULL DEFAULT '{}'::data_history_item[],
				activity_stamps int[] NOT NULL DEFAULT '{}'::int[],
				last_self_params_error text,
				CHECK (length(ID) = 32),
				PRIMARY KEY (id, date)
			);

			-- 11 (4 index)

			INSERT INTO storjinfo.nodes_history (id, date, free_data_items) (
				SELECT id, ((item).stamp AT TIME ZONE 'UTC')::date as date, array_agg(item ORDER BY (item).stamp) AS free_data_items FROM (
					SELECT id, unnest(free_data_items) AS item FROM nodes_history_old
				) AS t
				GROUP BY (id, date)
			);

			-- 64 (11+26=37 ? 27)

			INSERT INTO storjinfo.nodes_history (id, date, activity_stamps) (
				SELECT id, date, array_agg(stamp ORDER BY stamp) AS activity_stamps FROM (
					SELECT id, (to_timestamp(stamp & ~1) AT TIME ZONE 'UTC')::date as date, stamp FROM (
						SELECT id, unnest(activity_stamps) AS stamp FROM nodes_history_old
					) AS t
				) AS t
				GROUP BY (id, date)
			)
			ON CONFLICT (id, date) DO UPDATE SET activity_stamps = EXCLUDED.activity_stamps;

			-- 104 (11+26+58=95 ? 9)

			INSERT INTO storjinfo.nodes_history (id, date, last_self_params_error) (
				SELECT DISTINCT ON (id, date) id, date, last_self_params_error FROM (
					SELECT id, last_self_params_error, (to_timestamp(stamp & ~1) AT TIME ZONE 'UTC')::date as date FROM (
						SELECT id, last_self_params_error, unnest(activity_stamps) AS stamp FROM nodes_history_old
					) AS t
				) AS t
				ORDER BY id, date, date DESC
			)
			ON CONFLICT (id, date) DO UPDATE SET last_self_params_error = EXCLUDED.last_self_params_error;

			-- 114

			DROP TABLE storjinfo.nodes_history_old;
			`)
	}, func(db migrations.DB) error {
		return errors.New("migration not reveseable")
	})
}
