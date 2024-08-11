package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/rs/zerolog/log"
)

func UpdateASInfoIfNeed(db *pg.DB, asn int64) (bool, error) {
	var t int64
	_, err := db.Query(&t, `
		SELECT 1 FROM autonomous_systems
		WHERE number = ? AND incolumitas IS NOT NULL AND incolumitas_updated_at > NOW() - INTERVAL '7 days'`,
		asn)
	if t == 1 {
		return false, nil
	}
	if err != nil {
		return false, merry.Wrap(err)
	}

	info, err := fetchASInfo(asn)
	if err != nil {
		return false, merry.Wrap(err)
	}

	if len(info.Prefixes) == 0 {
		log.Warn().Int64("asn", asn).Msg("empty prefixes list")
	}
	if err := UpdateASPrefixes(db, asn, "incolumitas", info.Prefixes); err != nil {
		return false, merry.Wrap(err)
	}

	_, err = db.Exec(`
		INSERT INTO autonomous_systems (number, incolumitas, incolumitas_updated_at)
		VALUES (?, ?, NOW())
		ON CONFLICT (number) DO UPDATE SET
			incolumitas = EXCLUDED.incolumitas,
			incolumitas_updated_at = EXCLUDED.incolumitas_updated_at`,
		asn, info.asInfoToSave)
	if err != nil {
		return false, merry.Wrap(err)
	}
	return true, nil
}

func UpdateASPrefixes[T fmt.Stringer](db *pg.DB, asn int64, source string, prefixes []T) error {
	prefixesStr := make([]string, len(prefixes))
	for i, pref := range prefixes {
		prefixesStr[i] = pref.String()
	}

	_, err := db.Exec(`
		INSERT INTO autonomous_systems_prefixes (prefix, number, source)
			SELECT unnest(?::cidr[]), ?, ?
		ON CONFLICT (prefix, number, source) DO UPDATE SET
			updated_at = EXCLUDED.updated_at`,
		pg.Array(prefixesStr), asn, source)
	if err != nil {
		return merry.Wrap(err)
	}
	_, err = db.Exec(`
		DELETE FROM autonomous_systems_prefixes WHERE number = ? AND source = ? AND prefix NOT IN (?)`,
		asn, source, pg.In(prefixesStr))
	if err != nil {
		return merry.Wrap(err)
	}
	return nil
}

type asInfoToSave struct {
	Org    string `json:"org"`
	Descr  string `json:"descr"`
	Type   string `json:"type"`
	Domain string `json:"domain"`
}

type asInfoPrefix netip.Prefix

func (n *asInfoPrefix) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return merry.Wrap(err)
	}

	ipnet, err := netip.ParsePrefix(str)
	if err != nil {
		return merry.Wrap(err)
	}

	*n = asInfoPrefix(ipnet)
	return nil
}

func (n asInfoPrefix) String() string {
	return netip.Prefix(n).String()
}

type asInfoResponse struct {
	asInfoToSave
	Prefixes []asInfoPrefix `json:"prefixes"`
	// Asn int64 `json:"asn,omitempty"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

func fetchASInfo(asn int64) (asInfoResponse, error) {
	req, err := http.NewRequest("GET", "https://api.incolumitas.com/?q=AS"+strconv.FormatInt(asn, 10), nil)
	if err != nil {
		return asInfoResponse{}, merry.Wrap(err)
	}
	httpClient := http.Client{
		Timeout: 2 * time.Second,
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return asInfoResponse{}, merry.Wrap(err)
	}
	defer resp.Body.Close()

	info := asInfoResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return asInfoResponse{}, merry.Wrap(err)
	}
	if info.Error != "" {
		return asInfoResponse{}, merry.Errorf("ASN %d: %s: %s", asn, info.Error, info.Message)
	}

	log.Debug().Int64("ASN", asn).Str("org", info.Org).Str("type", info.Type).Msg("fetched AS type from incolumitas.com")
	return info, nil
}
