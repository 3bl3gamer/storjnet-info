package main

import "github.com/go-pg/migrations"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjinfo.nodes (
				id bytea PRIMARY KEY,
				created_at timestamptz NOT NULL DEFAULT NOW(),
				kad_params jsonb,
				kad_updated_at timestamptz,
				kad_checked_at timestamptz,
				self_params jsonb,
				self_updated_at timestamptz,
				self_checked_at timestamptz,
				CHECK (length(ID) = 32)
			)
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjinfo.nodes
			`)
	})
}
