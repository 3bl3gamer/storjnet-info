package main

import "github.com/go-pg/migrations/v8"

// Some stats sources name satellites by "host:port" only (stats.storkspace.net since 2026-11),
// so there may be no satellite_id to save. The length check still holds for non-NULL values.
func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE off_data_stats ALTER COLUMN satellite_id DROP NOT NULL;
			ALTER TABLE off_data_public_stats ALTER COLUMN satellite_id DROP NOT NULL;
			ALTER TABLE off_node_stats ALTER COLUMN satellite_id DROP NOT NULL;
			ALTER TABLE off_account_stats ALTER COLUMN satellite_id DROP NOT NULL;
			`)
	}, func(db migrations.DB) error {
		// rows without the id are removed: there is nothing to put in their place
		return execSome(db, `
			DELETE FROM off_data_stats WHERE satellite_id IS NULL;
			DELETE FROM off_data_public_stats WHERE satellite_id IS NULL;
			DELETE FROM off_node_stats WHERE satellite_id IS NULL;
			DELETE FROM off_account_stats WHERE satellite_id IS NULL;

			ALTER TABLE off_data_stats ALTER COLUMN satellite_id SET NOT NULL;
			ALTER TABLE off_data_public_stats ALTER COLUMN satellite_id SET NOT NULL;
			ALTER TABLE off_node_stats ALTER COLUMN satellite_id SET NOT NULL;
			ALTER TABLE off_account_stats ALTER COLUMN satellite_id SET NOT NULL;
			`)
	})
}
