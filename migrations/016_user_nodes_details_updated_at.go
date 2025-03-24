package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE user_nodes ADD COLUMN details_updated_at timestamptz NOT NULL DEFAULT now();
			UPDATE user_nodes SET details_updated_at = created_at;
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE user_nodes DROP COLUMN details_updated_at;
			`)
	})
}
