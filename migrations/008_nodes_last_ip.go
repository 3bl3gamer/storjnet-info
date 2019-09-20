package main

import "github.com/go-pg/migrations/v7"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE storjinfo.nodes
			ADD COLUMN last_ip inet
			`, `
			UPDATE storjinfo.nodes
			SET last_ip = (kad_params->>'last_ip')::inet
			WHERE kad_params ? 'last_ip'
			`, `
			CREATE FUNCTION node_last_ip_subnet(ip inet)
				RETURNS cidr
			AS
			$BODY$
				SELECT set_masklen(ip::cidr, 24);
			$BODY$
			LANGUAGE sql
			IMMUTABLE
			`, `
			CREATE INDEX nodes__last_ip_subnet__index ON nodes (node_last_ip_subnet(last_ip))
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP INDEX nodes__last_ip_subnet__index
			`, `
			DROP FUNCTION node_last_ip_subnet
			`, `
			ALTER TABLE storjinfo.nodes
			DROP COLUMN last_ip
			`)
	})
}
