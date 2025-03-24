package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE node_stats ADD COLUMN ip_types_asn_tops jsonb NOT NULL DEFAULT '{}'::jsonb;
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE node_stats DROP COLUMN ip_types_asn_tops;
			`)
	})
}
