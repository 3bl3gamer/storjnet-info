package main

import "github.com/go-pg/migrations/v7"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjnet.off_data_stats (
				id serial PRIMARY KEY,
				created_at timestamptz NOT NULL DEFAULT now(),
				satellite_id bytea NOT NULL,
				satellite_host text NOT NULL,
				bandwidth_bytes_downloaded bigint,
				bandwidth_bytes_uploaded bigint,
				storage_inline_bytes bigint,
				storage_inline_segments bigint,
				storage_median_healthy_pieces_count bigint,
				storage_min_healthy_pieces_count bigint,
				storage_remote_bytes bigint,
				storage_remote_segments bigint,
				storage_remote_segments_lost bigint,
				storage_total_bytes bigint,
				storage_total_objects bigint,
				storage_total_pieces bigint,
				storage_total_segments bigint,
				storage_free_capacity_estimate_bytes bigint,
				CHECK (length(satellite_id) = 32)
			);
			CREATE TABLE storjnet.off_node_stats (
				id serial PRIMARY KEY,
				created_at timestamptz NOT NULL DEFAULT now(),
				satellite_id bytea NOT NULL,
				satellite_host text NOT NULL,
				active_nodes int,
				disqualified_nodes int,
				exited_nodes int,
				offline_nodes int,
				suspended_nodes int,
				total_nodes int,
				vetted_nodes int,
				CHECK (length(satellite_id) = 32)
			);
			CREATE TABLE storjnet.off_account_stats (
				id serial PRIMARY KEY,
				satellite_id bytea NOT NULL,
				satellite_host text NOT NULL,
				created_at timestamptz NOT NULL DEFAULT now(),
				registered_accounts int,
				CHECK (length(satellite_id) = 32)
			);
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjnet.off_data_stats;
			DROP TABLE storjnet.off_node_stats;
			DROP TABLE storjnet.off_account_stats;
			`)
	})
}
