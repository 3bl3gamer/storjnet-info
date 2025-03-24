package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE node_stats ADD COLUMN subnets_count integer NOT NULL DEFAULT 0;
			ALTER TABLE node_stats ADD COLUMN subnet_countries jsonb NOT NULL DEFAULT '{}'::jsonb;
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE node_stats DROP COLUMN subnets_count;
			ALTER TABLE node_stats DROP COLUMN subnet_countries;
			`)
	})
}
