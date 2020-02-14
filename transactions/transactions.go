package transactions

import (
	"encoding/hex"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"net/url"
	"storjnet/utils"
	"strconv"
	"strings"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
)

type StorjTokenTransaction struct {
	Hash        [32]byte
	BlockNumber int32
	CreatedAt   time.Time
	AddrFrom    [20]byte
	AddrTo      [20]byte
	Value       float64 `pg:",use_zero"`
}

func decodeHex(buf []byte, str string) error {
	if !strings.HasPrefix(str, "0x") {
		return merry.New("expected " + str + " to have '0x' prefix")
	}
	str = str[2:]
	if len(buf)*2 != len(str) {
		return merry.Errorf("lengths mismatch: len(%s)=%d, len(buf)*2=%d", str, len(str), len(buf)*2)
	}
	if _, err := hex.Decode(buf, []byte(str)); err != nil {
		return merry.Wrap(err)
	}
	return nil
}

func decodeHexAddresses(strAddrs []string) ([][20]byte, error) {
	addrs := make([][20]byte, len(strAddrs))
	for i, strAddr := range strAddrs {
		if err := decodeHex(addrs[i][:], strAddr); err != nil {
			return nil, merry.Wrap(err)
		}
	}
	return addrs, nil
}

type EtherscanTx struct {
	Hash         string
	BlockNumber  string
	TimeStamp    string
	From         string
	To           string
	Value        string
	TokenDecimal string
	TokenSymbol  string
}

func (tx EtherscanTx) StorjTx() (storjTx StorjTokenTransaction, err error) {
	if tx.TokenSymbol != "STORJ" {
		return storjTx, merry.New("not Storj token: " + tx.TokenSymbol)
	}

	if err := decodeHex(storjTx.Hash[:], tx.Hash); err != nil {
		return storjTx, merry.Wrap(err)
	}

	bn, err := strconv.ParseInt(tx.BlockNumber, 10, 32)
	if err != nil {
		return storjTx, merry.Wrap(err)
	}
	storjTx.BlockNumber = int32(bn)

	ts, err := strconv.ParseInt(tx.TimeStamp, 10, 64)
	if err != nil {
		return storjTx, merry.Wrap(err)
	}
	storjTx.CreatedAt = time.Unix(ts, 0)

	if err := decodeHex(storjTx.AddrFrom[:], tx.From); err != nil {
		return storjTx, merry.Wrap(err)
	}

	if err := decodeHex(storjTx.AddrTo[:], tx.To); err != nil {
		return storjTx, merry.Wrap(err)
	}

	val, err := strconv.ParseFloat(tx.Value, 64)
	if err != nil {
		return storjTx, merry.Wrap(err)
	}
	dec, err := strconv.ParseInt(tx.TokenDecimal, 10, 64)
	if err != nil {
		return storjTx, merry.Wrap(err)
	}
	storjTx.Value = val / math.Pow10(int(dec))
	return
}

type EtherscanResult struct {
	Status  string
	Message string
	Result  json.RawMessage
}

type txFetchSummary struct {
	hasMore        bool
	firstCreatedAt time.Time
}

func fetchTransactionsFrom(db *pg.DB, addr [20]byte) (sum txFetchSummary, err error) {
	sum.firstCreatedAt = time.Now()

	apiKey, err := utils.RequireEnv("ETHERSCAN_API_KEY")
	if err != nil {
		return sum, merry.Wrap(err)
	}

	var curBlockNum int32
	_, err = db.QueryOne(&curBlockNum,
		`SELECT last_block_number FROM storj_token_addresses WHERE addr = ?`, addr[:])
	if err != nil && err != pg.ErrNoRows {
		return sum, merry.Wrap(err)
	}

	query := make(url.Values)
	query.Set("module", "account")
	query.Set("action", "tokentx")
	query.Set("apikey", apiKey)
	query.Set("address", "0x"+hex.EncodeToString(addr[:]))
	query.Set("startblock", strconv.FormatInt(int64(curBlockNum-1), 10))
	query.Set("endblock", "99999999")
	// query.Set("page", "1")
	// query.Set("offset", "0")
	query.Set("sort", "asc")
	resp, err := http.Get("https://api.etherscan.io/api?" + query.Encode())
	if err != nil {
		return sum, merry.Wrap(err)
	}
	defer resp.Body.Close()

	var res EtherscanResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return sum, merry.Wrap(err)
	}
	if res.Message != "OK" {
		return sum, merry.Errorf("api.etherscan.io error: %s (%s): %s", res.Message, res.Status, string(res.Result))
	}
	var transactions []*EtherscanTx
	if err := json.Unmarshal(res.Result, &transactions); err != nil {
		return sum, merry.Wrap(err)
	}
	var lastBlockNum int32

	err = db.RunInTransaction(func(tx *pg.Tx) error {
		for _, rawTx := range transactions {
			if rawTx.TokenSymbol != "STORJ" {
				continue
			}
			storjTx, err := rawTx.StorjTx()
			if err != nil {
				return merry.Wrap(err)
			}
			if _, err := tx.Model(&storjTx).OnConflict("(hash) DO NOTHING").Insert(); err != nil {
				return merry.Wrap(err)
			}
			if storjTx.BlockNumber > lastBlockNum {
				lastBlockNum = storjTx.BlockNumber
			}
			if storjTx.CreatedAt.Before(sum.firstCreatedAt) {
				sum.firstCreatedAt = storjTx.CreatedAt
			}
		}
		return nil
	})
	if err != nil {
		return sum, merry.Wrap(err)
	}

	_, err = db.Exec(`
		INSERT INTO storj_token_addresses (addr, last_block_number) VALUES (?,?)
		ON CONFLICT (addr) DO UPDATE SET last_block_number = EXCLUDED.last_block_number, updated_at = NOW()`,
		addr[:], lastBlockNum)
	if err != nil {
		return sum, merry.Wrap(err)
	}
	log.Printf("fetched: 0x%s, block: %d -> %d, tx: %d", hex.EncodeToString(addr[:]), curBlockNum, lastBlockNum, len(transactions))
	sum.hasMore = len(transactions) == 10000
	return sum, nil
}

func updateDaySummary(db *pg.DB, storjAddrs [][20]byte, date time.Time) error {
	date = date.In(time.UTC)
	if date.Hour() != 0 || date.Minute() != 0 || date.Second() != 0 || date.Nanosecond() != 0 {
		return merry.Errorf("not UTC day start: %s", date)
	}
	log.Print("updating TX summary on " + date.Format("2006-01-02"))
	_, err := db.Exec(`
		INSERT INTO storj_token_tx_summaries
		(date, preparings, payouts, payout_counts, withdrawals) (
			SELECT ?1::date, array_agg(preparings), array_agg(payouts), array_agg(payout_counts), array_agg(withdrawals)
			FROM (
				SELECT day,
					coalesce(sum(value) FILTER (WHERE addr_to IN (?0)), 0) AS preparings,
					coalesce(sum(value) FILTER (WHERE addr_from IN (?0)), 0) AS payouts,
					count(value)            FILTER (WHERE addr_from IN (?0)) AS payout_counts,
					coalesce(sum(value) FILTER (WHERE addr_from NOT IN (?0) AND addr_to NOT IN (?0)), 0) AS withdrawals
				FROM (
					SELECT extract('epoch' FROM (created_at - ?1))::int/3600 AS day, value, addr_from, addr_to
					FROM storj_token_transactions
					WHERE created_at >= ?1 AND created_at < (?1::timestamptz + INTERVAL '1 day')
					UNION SELECT generate_series(0, 23), 0, NULL, NULL
				) AS t
				GROUP BY day
				ORDER BY day
			) AS t
		)
		ON CONFLICT (date) DO UPDATE SET
			preparings = EXCLUDED.preparings,
			payouts = EXCLUDED.payouts,
			payout_counts = EXCLUDED.payout_counts,
			withdrawals = EXCLUDED.withdrawals`,
		pg.In(storjAddrs), date)
	return merry.Wrap(err)
}

func FetchAndProcess() error {
	db := utils.MakePGConnection()

	storjStrAddrs := []string{
		"0x005f7b5faa2f8a7a647d2b2dd2c278b35429fdc6",
		"0x00f5010ee550d6c58eb263bd46c5b9ab77943f8e",
		"0x004374c9d59a9b34cb6298f7906e126cb3c50c70",
		"0x0071edcf0c4dd52231bcafc7caab231062b75561",
	}
	storjAddrs, err := decodeHexAddresses(storjStrAddrs)
	if err != nil {
		return merry.Wrap(err)
	}

	var addrs [][20]byte
	_, err = db.Query(&addrs, `SELECT addr FROM storj_token_addresses ORDER BY updated_at ASC`)
	if err != nil {
		return merry.Wrap(err)
	}

	for _, storjAddr := range storjAddrs {
		found := false
		for _, addr := range addrs {
			if addr == storjAddr {
				found = true
				break
			}
		}
		if !found {
			addrs = append(addrs, storjAddr)
		}
	}

	firstCreatedAt := time.Now()
	for _, addr := range addrs {
		for {
			summary, err := fetchTransactionsFrom(db, addr)
			if err != nil {
				return merry.Wrap(err)
			}
			if summary.firstCreatedAt.Before(firstCreatedAt) {
				firstCreatedAt = summary.firstCreatedAt
			}
			if !summary.hasMore {
				break
			}
		}
	}

	_, err = db.Exec(`
		INSERT INTO storj_token_addresses (addr, last_block_number)
		(SELECT DISTINCT addr_to, 0 AS last_block_number FROM storj_token_transactions WHERE addr_from IN (?))
		ON CONFLICT DO NOTHING`,
		pg.In(storjAddrs))
	if err != nil {
		return merry.Wrap(err)
	}

	for date := firstCreatedAt.In(time.UTC).Truncate(24 * time.Hour); date.Before(time.Now()); date = date.AddDate(0, 0, 1) {
		if err := updateDaySummary(db, storjAddrs, date); err != nil {
			return merry.Wrap(err)
		}
	}
	return nil
}
