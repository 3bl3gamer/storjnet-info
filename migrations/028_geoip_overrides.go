package main

import "github.com/go-pg/migrations/v7"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjnet.geoip_overrides (
				network cidr PRIMARY KEY,
				location JSONB NOT NULL,
				created_at timestamptz NOT NULL DEFAULT now(),
				updated_at timestamptz NOT NULL DEFAULT now()
			);
			CREATE INDEX geoip_overrides__network__index ON geoip_overrides USING GIST(network inet_ops);
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP INDEX geoip_overrides__network__index;
			DROP TABLE storjnet.geoip_overrides;
			`)
	})
}
