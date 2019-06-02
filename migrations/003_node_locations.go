package main

import "github.com/go-pg/migrations"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TYPE storjinfo.node_location AS (
				country text,
				city text,
				longitude real,
				latitude real
			)
			`, `
			ALTER TABLE storjinfo.nodes ADD COLUMN location node_location
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE storjinfo.nodes DROP COLUMN location
			`, `
			DROP TYPE storjinfo.node_location
			`)
	})
}
