package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjnet.versions (
				kind text NOT NULL,
				version text NOT NULL,
				created_at timestamptz NOT NULL DEFAULT NOW()
			)
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjnet.versions
			`)
	})
}
