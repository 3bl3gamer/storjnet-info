package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjnet.nodes (
				id bytea PRIMARY KEY,
				ip_addr inet NOT NULL,
				port integer NOT NULL,
				location jsonb,
				created_at timestamptz NOT NULL DEFAULT NOW(),
				last_received_from_sat_at timestamptz NOT NULL,
				updated_at timestamptz,
				checked_at timestamptz,
				CHECK (length(id) = 32),
				CHECK (port > 0 AND port <= 65535)
			);
			CREATE TABLE storjnet.nodes_sat_offers (
				node_id bytea,
				satellite_name text,
				stamps timestamptz[] NOT NULL DEFAULT '{}',
				PRIMARY KEY (node_id, satellite_name),
				CHECK (length(node_id) = 32),
				CHECK (length(satellite_name) > 0)
			);
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjnet.nodes_sat_offers;
			DROP TABLE storjnet.nodes;
			`)
	})
}
