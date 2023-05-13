package main

import "github.com/go-pg/migrations/v7"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjnet.autonomous_systems;

			CREATE TABLE storjnet.autonomous_systems (
				number bigint PRIMARY KEY,

				incolumitas jsonb,
				incolumitas_updated_at timestamptz NOT NULL DEFAULT now(),

				ipinfo jsonb,
				ipinfo_updated_at timestamptz NOT NULL DEFAULT now(),

				created_at timestamptz NOT NULL DEFAULT now()
			);

			ALTER TABLE nodes DROP COLUMN ip_type;
			ALTER TABLE nodes ADD COLUMN asn bigint;
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjnet.autonomous_systems;

			CREATE TABLE storjnet.autonomous_systems (
				number bigint PRIMARY KEY,
				name text CHECK(name IS NULL OR name != ''),
				type text CHECK(type IS NULL OR type != ''),
				created_at timestamptz NOT NULL DEFAULT now(),
				updated_at timestamptz NOT NULL DEFAULT now()
			);

			ALTER TABLE nodes ADD COLUMN ip_type text;
			ALTER TABLE nodes DROP COLUMN asn;
			`)
	})
}
