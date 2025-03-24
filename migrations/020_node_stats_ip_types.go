package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE nodes ADD COLUMN ip_type text;

			CREATE TABLE storjnet.autonomous_systems (
				number bigint PRIMARY KEY,
				name text CHECK(name IS NULL OR name != ''),
				type text CHECK(type IS NULL OR type != ''),
				created_at timestamptz NOT NULL DEFAULT now(),
				updated_at timestamptz NOT NULL DEFAULT now()
			);

			ALTER TABLE node_stats ADD COLUMN ip_types jsonb NOT NULL DEFAULT '{}'::jsonb;
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE nodes DROP COLUMN ip_type;

			DROP TABLE storjnet.autonomous_systems;

			ALTER TABLE node_stats DROP COLUMN ip_types;
			`)
	})
}
