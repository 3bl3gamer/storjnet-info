package main

import "github.com/go-pg/migrations"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjinfo.global_stats (
				id serial PRIMARY KEY,
				created_at timestamptz NOT NULL DEFAULT NOW(),
				count_total int NOT NULL,
				count_active_24h int NOT NULL,
				count_active_12h int NOT NULL,
				versions jsonb,
				countries jsonb
			)
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjinfo.global_stats
			`)
	})
}
