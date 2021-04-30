package main

import "github.com/go-pg/migrations/v7"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE nodes ADD COLUMN tcp_updated_at timestamptz;
			ALTER TABLE nodes ADD COLUMN quic_updated_at timestamptz;

			ALTER TABLE node_stats ADD COLUMN active_count_proto jsonb NOT NULL default '{}';

			CREATE INDEX nodes__updated_at__index ON nodes (updated_at);
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP INDEX nodes__updated_at__index;

			ALTER TABLE node_stats DROP COLUMN active_count_proto;

			ALTER TABLE nodes DROP COLUMN tcp_updated_at;
			ALTER TABLE nodes DROP COLUMN quic_updated_at;
			`)
	})
}
