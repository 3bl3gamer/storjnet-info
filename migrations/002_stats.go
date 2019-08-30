package main

import "github.com/go-pg/migrations/v7"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TYPE storjinfo.activity_stat_item AS (hours int, count int);
			CREATE TYPE storjinfo.data_stat_item AS (percentile real, bytes_count bigint);
			CREATE TYPE storjinfo.version_stat_item AS (version text, count int);
			CREATE TYPE storjinfo.country_stat_item AS (country text, count int);
			CREATE TYPE storjinfo.difficulty_stat_item AS (difficulty int, count int);

			CREATE TABLE storjinfo.global_stats (
				id serial PRIMARY KEY,
				created_at timestamptz NOT NULL DEFAULT NOW(),
				count_total int NOT NULL,
				count_hours activity_stat_item[],
				free_disk      data_stat_item[],
				free_bandwidth data_stat_item[],
				versions     version_stat_item[],
				countries    country_stat_item[],
				difficulties difficulty_stat_item[]
			)
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjinfo.storjinfo.global_stats;
			DROP TYPE storjinfo.activity_stat_item;
			DROP TYPE storjinfo.data_stat_item;
			DROP TYPE storjinfo.version_stat_item;
			DROP TYPE storjinfo.country_stat_item;
			DROP TYPE storjinfo.difficulty_stat_item;
			`)
	})
}
