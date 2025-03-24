package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjnet.user_texts (
				user_id integer NOT NULL REFERENCES storjnet.users (id),
				date date NOT NULL,
				text text NOT NULL,
				updated_at timestamptz NOT NULL DEFAULT NOW(),
				PRIMARY KEY (user_id, date)
			)
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjnet.user_texts
			`)
	})
}
