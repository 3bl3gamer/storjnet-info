package main

import "github.com/go-pg/migrations"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE storjinfo.nodes_history
			RENAME COLUMN items TO free_data_items;

			ALTER TABLE storjinfo.nodes_history
			ALTER COLUMN free_data_items SET NOT NULL;

			ALTER TABLE storjinfo.nodes_history
			ALTER COLUMN free_data_items SET DEFAULT '{}'::data_history_item[];

			ALTER TABLE storjinfo.nodes_history
			ALTER COLUMN month_date SET NOT NULL;

			ALTER TABLE storjinfo.nodes_history
			ADD COLUMN activity_stamps int[] NOT NULL DEFAULT '{}'::int[];

			ALTER TABLE storjinfo.nodes_history
			ADD COLUMN last_self_params_error text
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE storjinfo.nodes_history
			RENAME COLUMN free_data_items TO items;

			ALTER TABLE storjinfo.nodes_history
			ALTER COLUMN items DROP NOT NULL;

			ALTER TABLE storjinfo.nodes_history
			ALTER COLUMN items DROP DEFAULT;

			ALTER TABLE storjinfo.nodes_history
			ALTER COLUMN month_date DROP NOT NULL;

			ALTER TABLE storjinfo.nodes_history
			DROP COLUMN activity_stamps;

			ALTER TABLE storjinfo.nodes_history
			DROP COLUMN last_self_params_error
			`)
	})
}
