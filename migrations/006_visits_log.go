package main

import "github.com/go-pg/migrations/v7"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjinfo.visits (
				visit_hash bytea NOT NULL,
				day_date date NOT NULL,
				visitor_hash bytea NOT NULL,
				user_agent text NOT NULL,
				path text NOT NULL,
				count int NOT NULL DEFAULT 1,
				CHECK (length(visit_hash) = 8),
				CHECK (length(visitor_hash) = 8),
				PRIMARY KEY (visit_hash, day_date)
			)
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjinfo.visits
			`)
	})
}
