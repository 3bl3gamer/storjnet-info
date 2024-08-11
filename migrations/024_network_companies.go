package main

import "github.com/go-pg/migrations/v7"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjnet.network_companies (
				id serial PRIMARY KEY,
				ip_from inet NOT NULL,
				ip_to inet NOT NULL,
				incolumitas jsonb,
				incolumitas_updated_at timestamptz,
				created_at timestamptz NOT NULL DEFAULT now()
			);
			CREATE INDEX network_companies__ip_range__index ON network_companies (ip_from ASC, ip_to DESC);

			CREATE TYPE storjnet.autonomous_system_info_source AS ENUM ('incolumitas', 'ipinfo');

			CREATE TABLE storjnet.autonomous_systems_prefixes (
				prefix cidr NOT NULL,
				number bigint NOT NULL,
				source autonomous_system_info_source NOT NULL,
				created_at timestamptz NOT NULL DEFAULT now(),
				updated_at timestamptz NOT NULL DEFAULT now(),
				PRIMARY KEY (prefix, number, source)
			);
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjnet.network_companies;
			DROP TABLE storjnet.autonomous_systems_prefixes;
			DROP TYPE storjnet.autonomous_system_info_source;
			`)
	})
}
