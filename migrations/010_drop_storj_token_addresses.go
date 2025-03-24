package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storj_token_addresses
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjnet.storj_token_addresses (
				addr bytea PRIMARY KEY CHECK (length(addr) = 20),
				last_block_number int NOT NULL,
				created_at timestamptz NOT NULL DEFAULT NOW(),
				updated_at timestamptz NOT NULL DEFAULT NOW()
			)
			`)
	})
}
