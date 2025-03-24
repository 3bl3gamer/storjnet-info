package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjnet.node_stats (
				id serial PRIMARY KEY,
				created_at timestamptz NOT NULL DEFAULT NOW(),
				count_total integer NOT NULL,
				active_count_hours jsonb NOT NULL,
				all_sat_offers_count_hours jsonb NOT NULL,
				per_sat_offers_count_hours jsonb NOT NULL,
				countries jsonb NOT NULL,
				ports jsonb NOT NULL
			);
			CREATE TABLE storjnet.node_daily_stats (
				date date NOT NULL,
				kind text NOT NULL,
				node_ids bytea[] NOT NULL,
				come_node_ids bytea[] NOT NULL,
				left_node_ids bytea[] NOT NULL,
				PRIMARY KEY (date, kind)
			);
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjnet.node_stats;
			DROP TABLE storjnet.node_daily_stats;
			`)
	})
}
