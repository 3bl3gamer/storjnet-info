package main

import "github.com/go-pg/migrations/v7"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TYPE storjinfo.data_history_item AS (
				stamp timestamptz,
				free_disk bigint,
				free_bandwidth bigint
			);
			CREATE TABLE storjinfo.nodes_history (
				id bytea NOT NULL,
				month_date date NOT NULL,
				items data_history_item[] NOT NULL,
				CHECK (length(ID) = 32),
				CHECK (extract(day from month_date) = 1),
				PRIMARY KEY (id, month_date)
			)
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjinfo.nodes_history;
			DROP TYPE storjinfo.data_history_item;
			`)
	})
}
