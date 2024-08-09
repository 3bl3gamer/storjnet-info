package main

import "github.com/go-pg/migrations/v7"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE storjnet.user_nodes_history RENAME TO user_nodes_history__current;

			CREATE TABLE storjnet.user_nodes_history
				(LIKE storjnet.user_nodes_history__current INCLUDING ALL)
				PARTITION BY RANGE (date);

			ALTER TABLE storjnet.user_nodes_history
				ATTACH PARTITION storjnet.user_nodes_history__current
				DEFAULT;
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE storjnet.user_nodes_history DETACH PARTITION storjnet.user_nodes_history__current;
			DROP TABLE storjnet.user_nodes_history;
			ALTER TABLE storjnet.user_nodes_history__current RENAME TO user_nodes_history;
			`)
	})
}
