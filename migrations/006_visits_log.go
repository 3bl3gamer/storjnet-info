package main

import "github.com/go-pg/migrations"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjinfo.visits (
				id bytea PRIMARY KEY,
				day_date date NOT NULL,
				visitor_id bytea NOT NULL,
				user_agent text NOT NULL,
				path text NOT NULL,
				count int NOT NULL DEFAULT 1,
				CHECK (length(id) = 8),
				CHECK (length(visitor_id) = 8),
				UNIQUE (id, day_date)
			)
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjinfo.visits
			`)
	})
}
