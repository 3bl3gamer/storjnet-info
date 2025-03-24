package main

import "github.com/go-pg/migrations/v8"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE users ADD COLUMN last_seen_at timestamptz DEFAULT now();
			UPDATE users SET last_seen_at = greatest(
				created_at,
				(select max(created_at) from user_nodes where user_id=id),
				(select max(updated_at) from user_texts where user_id=id)
			);
			ALTER TABLE users ALTER COLUMN last_seen_at SET NOT NULL;
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			ALTER TABLE users DROP COLUMN last_seen_at;
			`)
	})
}
