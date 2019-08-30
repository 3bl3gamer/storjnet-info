package main

import "github.com/go-pg/migrations/v7"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TYPE storjinfo.node_location AS (
				country text,
				city text,
				longitude real,
				latitude real
			);

			CREATE TABLE storjinfo.nodes (
				id bytea PRIMARY KEY,
				created_at timestamptz NOT NULL DEFAULT NOW(),
				kad_params jsonb,
				kad_updated_at timestamptz,
				kad_checked_at timestamptz,
				self_params jsonb,
				self_updated_at timestamptz,
				self_checked_at timestamptz,
				location node_location,
				CHECK (length(ID) = 32)
			)
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjinfo.nodes;
			DROP TYPE storjinfo.node_location;
			`)
	})
}
