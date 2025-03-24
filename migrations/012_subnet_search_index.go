package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE FUNCTION storjnet.node_ip_subnet(ip inet)
			RETURNS inet AS $$
				SELECT set_masklen(ip::cidr, 24);
			$$ LANGUAGE SQL IMMUTABLE;
			CREATE INDEX nodes__ip_addr_subnet__index ON nodes (node_ip_subnet(ip_addr));
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP INDEX nodes__ip_addr_subnet__index;
			DROP FUNCTION storjnet.node_ip_subnet(inet);
			`)
	})
}
