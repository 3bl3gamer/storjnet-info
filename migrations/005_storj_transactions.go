package main

import "github.com/go-pg/migrations/v7"

func init() {
	migrations.MustRegisterTx(func(db migrations.DB) error {
		return execSome(db, `
			CREATE TABLE storjnet.storj_token_addresses (
				addr bytea PRIMARY KEY CHECK (length(addr) = 20),
				last_block_number int NOT NULL,
				created_at timestamptz NOT NULL DEFAULT NOW(),
				updated_at timestamptz NOT NULL DEFAULT NOW()
			);
			CREATE TABLE storjnet.storj_token_transactions (
				hash bytea PRIMARY KEY CHECK (length(hash) = 32),
				block_number int NOT NULL,
				created_at timestamptz NOT NULL,
				addr_from bytea NOT NULL CHECK (length(addr_from) = 20),
				addr_to bytea NOT NULL CHECK (length(addr_to) = 20),
				value float8 NOT NULL
			);
			CREATE INDEX storj_token_transactions__created_at ON storjnet.storj_token_transactions (created_at);
			CREATE TABLE storjnet.storj_token_tx_summaries (
				date date PRIMARY KEY,
				preparings float4[] NOT NULL CHECK (array_dims(preparings) = '[1:24]'),
				payouts float4[] NOT NULL CHECK (array_dims(payouts) = '[1:24]'),
				payout_counts int[] NOT NULL CHECK (array_dims(payout_counts) = '[1:24]'),
				withdrawals float4[] NOT NULL CHECK (array_dims(withdrawals) = '[1:24]')
			)
			`)
	}, func(db migrations.DB) error {
		return execSome(db, `
			DROP TABLE storjnet.storj_token_addresses;
			DROP TABLE storjnet.storj_token_transactions;
			DROP TABLE storjnet.storj_token_tx_summaries;
			`)
	})
}
