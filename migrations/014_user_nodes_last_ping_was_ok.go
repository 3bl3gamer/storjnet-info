package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE user_nodes ADD COLUMN last_ping_was_ok bool;
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE user_nodes DROP COLUMN last_ping_was_ok;
			`)
	})
}
