package main

import "github.com/go-pg/migrations/v7"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjnet.users (
				id serial PRIMARY KEY,
				email text NOT NULL UNIQUE,
				username text UNIQUE,
				password_hash text NOT NULL,
				sessid text NOT NULL,
				created_at timestamptz NOT NULL DEFAULT NOW()
			);

			CREATE TYPE storjnet.node_ping_mode AS ENUM ('off', 'dial', 'ping');

			CREATE TABLE storjnet.user_nodes (
				node_id bytea NOT NULL,
				user_id integer NOT NULL REFERENCES storjnet.users (id),
				address text NOT NULL,
				created_at timestamptz NOT NULL DEFAULT NOW(),
				ping_mode storjnet.node_ping_mode NOT NULL DEFAULT 'off',
				CHECK (length(node_id) = 32),
				PRIMARY KEY (node_id, user_id)
			);

			CREATE TABLE storjnet.user_nodes_history (
				id bytea NOT NULL,
				user_id integer NOT NULL,
				date date NOT NULL,
				activity_stamps int[] NOT NULL DEFAULT '{}'::int[],
				CHECK (length(ID) = 32),
				PRIMARY KEY (id, user_id, date),
				FOREIGN KEY (id, user_id) REFERENCES storjnet.user_nodes (node_id, user_id)
			);
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjnet.user_nodes_history;
			DROP TABLE storjnet.user_nodes;
			DROP TYPE storjnet.node_ping_mode;
			DROP TABLE storjnet.users;
			`)
	})
}
