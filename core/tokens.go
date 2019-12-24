package core

import "time"

type StorjTokenTxSummary struct {
	Date         time.Time
	Preparings   []float32 `pg:",array"`
	Payouts      []float32 `pg:",array"`
	PayoutCounts []int32   `pg:",array"`
	Withdrawals  []float32 `pg:",array"`
}
