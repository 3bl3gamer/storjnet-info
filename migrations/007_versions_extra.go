package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE storjnet.versions ADD COLUMN extra jsonb
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE storjnet.versions DROP COLUMN extra
			`)
	})
}
