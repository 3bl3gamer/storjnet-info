package core

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v10"
)

// previously was api.incolumitas.com
const ipapiURL = "https://api.ipapi.is/"

// Since 2026-09-01 ipapi.is requires an API key for everything except plain IP ownership
// and geolocation: ASN queries are refused with 403 and IP responses come back without
// the company object (name, domain, type, network). A free key allows 1000 requests per day.
const ipapiKeyEnvName = "IPAPI_KEY"

// First 1000 requests/day are free (resets at midnight UTC), trying to fit into that.
const ipapiFreeDailyLimit = 1000
const ipapiCounterKey = "ipapi:daily_requests"

var (
	ErrIncolumitasTooManyRequests = merry.New("ipapi: too many requests")
	ErrIncolumitasKeyNotSet       = merry.New("ipapi: " + ipapiKeyEnvName + " env variable is not set")
	ErrIncolumitasNoData          = merry.New("ipapi: no data")
	ErrIPAPIDailyLimit            = merry.New("ipapi: daily requests limit reached")
)

// https://ipapi.is/developers.html#error-handling
type ipapiErrorResponse struct {
	Error string `json:"error"`
	// Stable and machine-readable, unlike the Error text, so it is the one to branch on.
	// Empty when the request itself was valid but nothing is known about the queried value.
	ErrorCode string `json:"error_code"`
}

// fetchIPAPI requests https://api.ipapi.is/?q=<query> and decodes the response into result.
// query is an IP address or an "AS<number>" string.
func fetchIPAPI(query string, timeout time.Duration, result interface{}) error {
	key, ok := os.LookupEnv(ipapiKeyEnvName)
	if !ok || key == "" {
		return ErrIncolumitasKeyNotSet.Here()
	}

	// The key may end up in the logs. Better use POST? TODO
	reqURL := ipapiURL + "?" + url.Values{"q": {query}, "key": {key}}.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return merry.Wrap(hideIPAPIKey(err, key))
	}
	httpClient := http.Client{Timeout: timeout}
	// dialer, err := proxy.SOCKS5("tcp", "127.0.0.1:9051", nil, proxy.Direct)
	// if err != nil {
	// 	return merry.Wrap(err)
	// }
	// httpClient.Transport = &http.Transport{DialContext: dialer.(proxy.ContextDialer).DialContext}

	resp, err := httpClient.Do(req)
	if err != nil {
		return merry.Wrap(hideIPAPIKey(err, key))
	}
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return merry.Wrap(hideIPAPIKey(err, key))
	}

	errResp := ipapiErrorResponse{}
	if err := json.Unmarshal(buf, &errResp); err != nil {
		return merry.Wrap(err)
	}
	if errResp.Error != "" {
		switch errResp.ErrorCode {
		case "ERR_QUOTA_EXCEEDED", "ERR_FREE_TIER_EXHAUSTED":
			return ErrIncolumitasTooManyRequests.Here().WithMessagef("%s: %s", query, errResp.Error)
		case "":
			// a valid query for something the API has no data about (an unknown ASN), comes with status 200
			return ErrIncolumitasNoData.Here().WithMessagef("%s: %s", query, errResp.Error)
		default:
			return merry.Errorf("ipapi: %s: %s: %s", query, errResp.ErrorCode, errResp.Error)
		}
	}
	if resp.StatusCode != http.StatusOK {
		bodyStart := buf
		if len(bodyStart) > 200 {
			bodyStart = bodyStart[:200]
		}
		return merry.Errorf("ipapi: %s: unexpected status %d: %s", query, resp.StatusCode, bodyStart)
	}

	return merry.Wrap(json.Unmarshal(buf, result))
}

type ipapiCounter struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

func ipapiToday() string {
	return time.Now().UTC().Format("2006-01-02") //ipapi free daily quota resets and midnight UTC
}

// checkIPAPIFreeDailyLimit returns ErrIPAPIDailyLimit if today's requests free limit is reached.
// The check and the increment are not atomic: concurrent fetchers may overshoot by a few requests.
func checkIPAPIFreeDailyLimit(db DBTx) error {
	var res struct{ Value ipapiCounter }
	err := AppConfigVal(db, ipapiCounterKey, &res, false)
	if err != nil && err != pg.ErrNoRows {
		return merry.Wrap(err)
	}
	if res.Value.Date == ipapiToday() && res.Value.Count >= ipapiFreeDailyLimit {
		return ErrIPAPIDailyLimit.Here().WithMessagef("%d requests today", res.Value.Count)
	}
	return nil
}

// countIPAPIRequestIfSent increments today's requests counter
// if the request was successful (returned fetchErr is nil).
// Timeouts and other errors are not counted.
func countIPAPIRequestIfSent(db DBTx, fetchErr error) error {
	// not sure about ErrIncolumitasNoData, let's assume it is counted as normal request
	if fetchErr != nil && !merry.Is(fetchErr, ErrIncolumitasNoData) {
		return nil
	}
	_, err := db.Exec(`
		INSERT INTO appconfig (key, value) VALUES (?, jsonb_build_object('date', ?::text, 'count', 1))
		ON CONFLICT (key) DO UPDATE SET value = CASE
			WHEN appconfig.value->>'date' = EXCLUDED.value->>'date'
			THEN jsonb_set(appconfig.value, '{count}', to_jsonb((appconfig.value->>'count')::int + 1))
			ELSE EXCLUDED.value
		END`,
		ipapiCounterKey, ipapiToday())
	return merry.Wrap(err)
}

func hideIPAPIKey(err error, key string) error {
	if err == nil {
		return nil
	}
	if msg := err.Error(); strings.Contains(msg, key) {
		return merry.New(strings.ReplaceAll(msg, key, "<"+ipapiKeyEnvName+">"))
	}
	return err
}
