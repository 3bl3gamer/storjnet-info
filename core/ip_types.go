package core

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/abh/geoip"
	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/rs/zerolog/log"
)

func FindCachedIPType(db *pg.DB, asndb *geoip.GeoIP, ipAddr string) (string, bool, error) {
	asnAndName, _ := asndb.GetName(ipAddr)
	if asnAndName == "" {
		return "", false, nil
	}

	asnAndNameSlice := strings.SplitN(asnAndName, " ", 2)
	asnStr := asnAndNameSlice[0]
	asnStr = strings.ToUpper(asnStr)
	if strings.HasPrefix(asnStr, "AS") {
		asnStr = asnStr[2:]
	}
	asn, err := strconv.ParseInt(asnStr, 10, 64)
	if err != nil {
		return "", false, merry.Prependf(err, "parsing AS number from '%s':", asnAndName)
	}

	type TypeWithStamp struct {
		Type      string
		UpdatedAt time.Time
	}
	curType := TypeWithStamp{}
	_, err = db.QueryOne(&curType, "SELECT type, updated_at FROM autonomous_systems WHERE number = ?", asn)
	if err != nil && err != pg.ErrNoRows {
		return "", false, merry.Wrap(err)
	}

	ipType := curType.Type
	if err == pg.ErrNoRows || curType.UpdatedAt.Before(time.Now().Add(-7*24*time.Hour)) {
		info, err := fetchASInfo(asn)
		if err != nil {
			log.Error().Err(err).Int64("ASN", asn).Msg("failed to get AS information")
		} else {
			ipType = orBlank(info.Type)
			_, err := db.Exec(`
				INSERT INTO autonomous_systems (number, name, type) VALUES (?, ?, ?)
				ON CONFLICT (number) DO UPDATE SET name = EXCLUDED.name, type = EXCLUDED.type, updated_at = NOW()`,
				asn, info.Org, info.Type)
			if err != nil {
				return "", false, merry.Wrap(err)
			}
		}
	}

	return ipType, ipType != "", nil
}

// org and type may be missing in response (unknown)
type asInfo struct {
	Org  *string
	Type *string
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
	log.Debug().Int64("ASN", asn).Str("org", orBlank(info.Org)).Str("type", orBlank(info.Type)).Msg("fetched AS type")
	return info, nil
}

func orBlank(str *string) string {
	if str == nil {
		return ""
	}
	return *str
}
