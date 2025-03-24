package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjnet.appconfig (
				key TEXT PRIMARY KEY,
				value JSONB NOT NULL
			)
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjnet.appconfig
			`)
	})
}
