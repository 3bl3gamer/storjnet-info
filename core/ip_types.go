package core

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/abh/geoip"
	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/rs/zerolog/log"
)

func FindIPAddrASN(asndb *geoip.GeoIP, ipAddr string) (int64, bool, error) {
	asnAndName, _ := asndb.GetName(ipAddr)
	if asnAndName == "" {
		return 0, false, nil
	}

	asnAndNameSlice := strings.SplitN(asnAndName, " ", 2)
	asnStr := asnAndNameSlice[0]
	asnStr = strings.ToUpper(asnStr)
	if strings.HasPrefix(asnStr, "AS") {
		asnStr = asnStr[2:]
	}
	asn, err := strconv.ParseInt(asnStr, 10, 64)
	if err != nil {
		return 0, false, merry.Prependf(err, "parsing AS number from '%s':", asnAndName)
	}
	return asn, true, nil
}

func UpdateASInfo(db *pg.DB, asn int64) (bool, error) {
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

	_, err = db.Exec(`
		INSERT INTO autonomous_systems (number, incolumitas, incolumitas_updated_at)
		VALUES (?, ?, NOW())
		ON CONFLICT (number) DO UPDATE SET
			incolumitas = EXCLUDED.incolumitas,
			incolumitas_updated_at = EXCLUDED.incolumitas_updated_at`,
		asn, info.asInfoShort)
	if err != nil {
		return false, merry.Wrap(err)
	}
	return true, nil
}

type asInfoShort struct {
	Org    string `json:"org,omitempty"`
	Descr  string `json:"descr,omitempty"`
	Type   string `json:"type,omitempty"`
	Domain string `json:"domain,omitempty"`
}
type asInfo struct {
	asInfoShort
	Asn int64 `json:"asn,omitempty"`
}

func fetchASInfo(asn int64) (asInfo, error) {
	req, err := http.NewRequest("GET", "https://api.incolumitas.com/?q=AS"+strconv.FormatInt(asn, 10), nil)
	if err != nil {
		return asInfo{}, merry.Wrap(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return asInfo{}, merry.Wrap(err)
	}
	defer resp.Body.Close()

	info := asInfo{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return asInfo{}, merry.Wrap(err)
	}
	log.Debug().Int64("ASN", asn).Str("org", info.Org).Str("type", info.Type).Msg("fetched AS type from incolumitas.com")
	return info, nil
}
