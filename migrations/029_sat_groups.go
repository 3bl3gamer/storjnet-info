package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE node_stats ADD COLUMN sat_group text NOT NULL DEFAULT 'all';
			CREATE INDEX node_stats__sat_group__created_at__index ON node_stats (sat_group, created_at);

			ALTER TABLE off_node_stats ADD COLUMN active_wallets int;
			ALTER TABLE off_node_stats ADD COLUMN active_select_nodes int;

			ALTER TABLE off_data_stats ADD COLUMN storage_total_bytes_after_expansion double precision;

			CREATE TABLE storjnet.off_data_public_stats (
				id serial PRIMARY KEY,
				created_at timestamptz NOT NULL DEFAULT now(),
				satellite_id bytea NOT NULL,
				satellite_host text NOT NULL,
				bandwidth_bytes_downloaded bigint,
				bandwidth_bytes_uploaded bigint,
				storage_total_bytes bigint,
				storage_total_objects bigint,
				storage_total_segments bigint,
				CHECK (length(satellite_id) = 32)
			);
			`)
	}, func(db migrations.DB) error {
		// rows of the groups are removed: without the column they would be indistinguishable from the old ones
		return execSome(db, `
			DROP TABLE storjnet.off_data_public_stats;
			ALTER TABLE off_data_stats DROP COLUMN storage_total_bytes_after_expansion;

			ALTER TABLE off_node_stats DROP COLUMN active_select_nodes;
			ALTER TABLE off_node_stats DROP COLUMN active_wallets;

			DELETE FROM node_stats WHERE sat_group != 'all';
			DROP INDEX node_stats__sat_group__created_at__index;
			ALTER TABLE node_stats DROP COLUMN sat_group;
			`)
	})
}
