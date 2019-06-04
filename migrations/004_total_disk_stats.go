package main

import "github.com/go-pg/migrations"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TYPE storjinfo.type_stat_item AS (type int, count int);

			ALTER TABLE storjinfo.global_stats
			ADD COLUMN free_disk_total data_stat_item[],
			ADD COLUMN types type_stat_item[]
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE storjinfo.global_stats
			DROP COLUMN free_disk_total,
			DROP COLUMN types;

			DROP TYPE type_stat_item
			`)
	})
}
