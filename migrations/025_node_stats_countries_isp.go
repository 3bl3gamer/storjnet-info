package main

import "github.com/go-pg/migrations/v7"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE node_stats ADD COLUMN countries_isp jsonb NOT NULL DEFAULT '{}'::jsonb;
			ALTER TABLE node_stats ADD COLUMN subnet_countries_isp jsonb NOT NULL DEFAULT '{}'::jsonb;
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE node_stats DROP COLUMN countries_isp;
			ALTER TABLE node_stats DROP COLUMN subnet_countries_isp;
			`)
	})
}
