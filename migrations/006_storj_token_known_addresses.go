package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjnet.storj_token_known_addresses (
				addr bytea PRIMARY KEY CHECK (length(addr) = 20),
				description text NOT NULL,
				kind text NOT NULL,
				proof text,
				created_at timestamptz NOT NULL DEFAULT NOW()
			)`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjnet.storj_token_known_addresses
			`)
	})
}
