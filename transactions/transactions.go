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
	AddrFrom    [20]byte `pg:",use_zero"`
	AddrTo      [20]byte `pg:",use_zero"`
	Value       float64  `pg:",use_zero"`
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

// func decodeHexAddresses(strAddrs []string) ([][20]byte, error) {
// 	addrs := make([][20]byte, len(strAddrs))
// 	for i, strAddr := range strAddrs {
// 		if err := decodeHex(addrs[i][:], strAddr); err != nil {
// 			return nil, merry.Wrap(err)
// 		}
// 	}
// 	return addrs, nil
// }

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
	lastCreatedAt  time.Time
}

func fetchTransactions(db *pg.DB) (sum txFetchSummary, err error) {
	sum.firstCreatedAt = time.Now()

	apiKey, err := utils.RequireEnv("ETHERSCAN_API_KEY")
	if err != nil {
		return sum, merry.Wrap(err)
	}

	var curBlockNum int64
	_, err = db.QueryOne(&curBlockNum,
		`SELECT max(block_number) FROM storj_token_transactions`)
	if err != nil && err != pg.ErrNoRows {
		return sum, merry.Wrap(err)
	}

	query := make(url.Values)
	query.Set("module", "account")
	query.Set("action", "tokentx")
	query.Set("apikey", apiKey)
	query.Set("contractaddress", "0xb64ef51c888972c908cfacf59b47c1afbc0ab8ac")
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
			// fmt.Printf("%#v\n%#v\n", rawTx, storjTx)
			if _, err := tx.Model(&storjTx).OnConflict("(hash) DO NOTHING").Insert(); err != nil {
				return merry.Wrap(err)
			}
			if storjTx.BlockNumber > lastBlockNum {
				lastBlockNum = storjTx.BlockNumber
			}
			if storjTx.CreatedAt.Before(sum.firstCreatedAt) {
				sum.firstCreatedAt = storjTx.CreatedAt
			}
			if storjTx.CreatedAt.After(sum.lastCreatedAt) {
				sum.lastCreatedAt = storjTx.CreatedAt
			}
		}
		return nil
	})
	if err != nil {
		return sum, merry.Wrap(err)
	}

	log.Printf("fetched, block: %d -> %d, count: %d, last: %s",
		curBlockNum, lastBlockNum, len(transactions), sum.lastCreatedAt.Format("2006-01-02 15:04"))
	sum.hasMore = len(transactions) == 10000
	return sum, nil
}

func updateDaySummary(db *pg.DB, date time.Time) error {
	date = date.In(time.UTC)
	if date.Hour() != 0 || date.Minute() != 0 || date.Second() != 0 || date.Nanosecond() != 0 {
		return merry.Errorf("not UTC day start: %s", date)
	}
	log.Print("updating TX summary on " + date.Format("2006-01-02"))

	err := db.RunInTransaction(func(tx *pg.Tx) error {
		var payoutAddrs [][20]byte
		_, err := tx.Query(&payoutAddrs,
			`SELECT addr FROM storj_token_known_addresses WHERE kind = 'payout'`)
		if err != nil {
			return merry.Wrap(err)
		}
		var payoutAddrsMap = make(map[[20]byte]struct{})
		for _, addr := range payoutAddrs {
			payoutAddrsMap[addr] = struct{}{}
		}

		// everythin except withdrawals

		_, err = tx.Exec(`
			INSERT INTO storj_token_tx_summaries
			(date, preparings, payouts, payout_counts, withdrawals) (
				SELECT ?0::date, array_agg(preparings), array_agg(payouts), array_agg(payout_counts), array_agg(withdrawals)
				FROM (
					SELECT day,
						coalesce(sum(value)   FILTER (WHERE addr_to   IN (?1) AND addr_from NOT IN (?1)), 0) AS preparings,
						coalesce(sum(value)   FILTER (WHERE addr_from IN (?1) AND addr_to   NOT IN (?1)), 0) AS payouts,
						coalesce(count(value) FILTER (WHERE addr_from IN (?1) AND addr_to   NOT IN (?1)), 0) AS payout_counts,
						0 AS withdrawals
					FROM (
						SELECT extract('epoch' FROM (created_at - ?0))::int/3600 AS day, value, addr_from, addr_to
						FROM storj_token_transactions
						WHERE created_at >= ?0 AND created_at < (?0::timestamptz + INTERVAL '1 day')
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
			date, pg.In(payoutAddrs))
		if err != nil {
			return merry.Wrap(err)
		}

		// withdrawals

		// all transactions intil this day (inclusve) which have received any payout
		var transactions []StorjTokenTransaction
		_, err = tx.Query(&transactions, `
			WITH receipt_addrs AS (
				SELECT DISTINCT addr_to
				FROM storj_token_transactions
				WHERE addr_from IN (?1)
				  AND addr_to NOT IN (?1)
				  AND created_at < ?0::timestamptz + INTERVAL '1 day'
			)
			SELECT created_at, addr_from, addr_to, value FROM storj_token_transactions
			WHERE created_at < ?0::timestamptz + INTERVAL '1 day'
			  AND (addr_to IN (SELECT * FROM receipt_addrs) OR addr_from IN (SELECT * FROM receipt_addrs))
			  AND addr_to NOT IN (?1)
			ORDER BY created_at`,
			date, pg.In(payoutAddrs))
		if err != nil {
			return merry.Wrap(err)
		}

		limits := make(map[[20]byte]float64) // total_received_payout - total_withdrawals
		hourWSums := make([]float64, 24)     // withdrawal sums per hours, will be save to "withdrawals" column

		for _, tx := range transactions {
			_, isFromPayout := payoutAddrsMap[tx.AddrFrom]

			// TXs from cur day
			if !isFromPayout && tx.CreatedAt.After(date) {
				delta := tx.Value + limits[tx.AddrFrom]
				if delta > 0 {
					hour := tx.CreatedAt.In(time.UTC).Hour()
					hourWSums[hour] += delta
				}
			}

			if isFromPayout {
				limits[tx.AddrTo] += tx.Value //payout from satellite
			} else {
				limits[tx.AddrFrom] -= tx.Value //withdrawal to somewhere
			}
		}

		_, err = tx.Exec(
			`UPDATE storj_token_tx_summaries SET withdrawals = ?1 WHERE date = ?0`,
			date, pg.Array(hourWSums))
		if err != nil {
			return merry.Wrap(err)
		}
		return nil
	})
	return merry.Wrap(err)
}

func FetchAndProcess(startDate time.Time) error {
	db := utils.MakePGConnection()

	for {
		summary, err := fetchTransactions(db)
		if err != nil {
			return merry.Wrap(err)
		}
		if !summary.hasMore {
			break
		}
	}

	if startDate.IsZero() {
		_, err := db.QueryOne(&startDate, `
		SELECT COALESCE(
			(SELECT max(date) FROM storj_token_tx_summaries),
			(SELECT (min(created_at) AT TIME ZONE 'UTC')::date FROM storj_token_transactions)
		)`)
		if err != nil {
			return merry.Wrap(err)
		}
	}

	for date := startDate.In(time.UTC); date.Before(time.Now()); date = date.AddDate(0, 0, 1) {
		if err := updateDaySummary(db, date); err != nil {
			return merry.Wrap(err)
		}
	}
	return nil
}
