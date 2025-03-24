package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE off_node_stats ADD COLUMN full_nodes bigint;
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE off_node_stats DROP COLUMN full_nodes;
			`)
	})
}
