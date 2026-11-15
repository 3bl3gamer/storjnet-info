package main

import "github.com/go-pg/migrations/v8"

// Coordinates of the active nodes for the map, prepared by stat-nodes: only the last state,
// one row per satellites group (and "all"). /api/nodes/locations returns coords as is.
func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjnet.node_last_locations (
				sat_group text PRIMARY KEY,
				created_at timestamptz NOT NULL,
				coords bytea NOT NULL
			);
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjnet.node_last_locations;
			`)
	})
}
