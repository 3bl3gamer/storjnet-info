package utils

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/lib/pq"
	"github.com/oschwald/geoip2-golang"
	"github.com/rs/zerolog/log"
)

func align(sql string) string {
	lines := strings.Split(sql, "\n")

	minIndent := 999
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		for i, char := range line {
			if char != '\t' {
				if i < minIndent {
					minIndent = i
				}
				break
			}
		}
	}

	sql = ""
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			sql += line[minIndent:]
		}
		sql += "\n"
	}
	return strings.TrimSpace(sql)
}

type dbLogger struct{}

func (d dbLogger) BeforeQuery(ctx context.Context, event *pg.QueryEvent) (context.Context, error) {
	return ctx, nil
}

func (d dbLogger) AfterQuery(ctx context.Context, event *pg.QueryEvent) error {
	query, err := event.FormattedQuery()
	if err != nil {
		return err
	}
	log.Printf("\033[36m%s\n\033[34m%s\033[39m", time.Since(event.StartTime), align(query))
	return nil
}

func MakePGConnection() *pg.DB {
	db := pg.Connect(&pg.Options{User: "storjnet", Password: "storj", Database: "storjnet_db"})
	// db.AddQueryHook(dbLogger{})
	return db
}

type GeoIPConn struct {
	value       atomic.Value
	fpath       string
	lastModTime time.Time
}

func OpenGeoIPConn(fpath string) (*GeoIPConn, error) {
	_, err := os.Stat(fpath)
	if err != nil && !os.IsNotExist(err) {
		return nil, merry.Wrap(err)
	}
	if os.IsNotExist(err) {
		if !filepath.IsAbs(fpath) {
			ex, err := os.Executable()
			if err != nil {
				return nil, merry.Wrap(err)
			}
			baseDir := filepath.Dir(ex)
			fpath = filepath.Clean(baseDir + "/" + fpath)
		}
	}

	geoip := &GeoIPConn{
		value: atomic.Value{},
		fpath: fpath,
	}

	if _, err := geoip.reopenIfUpdated(); err != nil {
		return nil, merry.Wrap(err)
	}

	go func() {
		for {
			reopened, err := geoip.reopenIfUpdated()
			if err != nil {
				log.Error().Err(err).Str("fpath", fpath).Msg("failed to reopen GeoIP DB, keep using old one")
			}
			if reopened {
				log.Debug().Str("fpath", fpath).Msg("reopened GeoIP DB connection")
			}
			time.Sleep(time.Minute)
		}
	}()

	return geoip, nil
}

func (c *GeoIPConn) reopenIfUpdated() (bool, error) {
	stat, err := os.Stat(c.fpath)
	if err != nil {
		return false, merry.Wrap(err)
	}

	if stat.ModTime().After(c.lastModTime) {
		conn, err := geoip2.Open(c.fpath)
		if err != nil {
			return false, merry.Wrap(err)
		}
		// no need to wait for old connection users and close it explictly:
		// the connection has finalizer and will be closed automatically
		// https://github.com/oschwald/geoip2-golang/issues/63#issuecomment-651125478
		// https://github.com/oschwald/maxminddb-golang/blob/main/reader_mmap.go#L50
		c.value.Store(conn)
		c.lastModTime = stat.ModTime()
		return true, nil
	}
	return false, nil
}

func (c *GeoIPConn) CityStr(ipStr string) (*geoip2.City, bool, error) {
	ipAddress := net.ParseIP(ipStr)
	if ipAddress == nil {
		return nil, false, merry.New("invalid IP: " + ipStr)
	}
	conn := c.value.Load().(*geoip2.Reader)
	city, err := conn.City(ipAddress)
	if err != nil {
		return nil, false, merry.Wrap(err)
	}
	if city.Country.IsoCode == "" {
		return nil, false, nil
	}
	return city, true, nil
}

func (c *GeoIPConn) ASNStr(ipStr string) (*geoip2.ASN, bool, error) {
	ipAddress := net.ParseIP(ipStr)
	if ipAddress == nil {
		return nil, false, merry.New("invalid IP: " + ipStr)
	}
	conn := c.value.Load().(*geoip2.Reader)
	asn, err := conn.ASN(ipAddress)
	if err != nil {
		return nil, false, merry.Wrap(err)
	}
	if asn.AutonomousSystemNumber == 0 {
		return nil, false, nil
	}
	return asn, true, nil
}

func (c *GeoIPConn) Close() {
	panic("not implemented")
}

func IsConstrError(err error, table, kind, name string) bool {
	if perr, ok := merry.Unwrap(err).(pg.Error); ok {
		return perr.Field('t') == table &&
			pq.ErrorCode(perr.Field('C')).Name() == kind &&
			perr.Field('n') == name
	}
	return false
}

func PrintPGErrInfo(err error) {
	perr := err.(pg.Error)
	for c := byte('A'); c <= byte('z'); c++ {
		if perr.Field(c) != "" {
			println(c, string(c), perr.Field(c))
		}
	}
}

func SaveChunked(db *pg.DB, chunkSize int, channel chan interface{}, handler func(tx *pg.Tx, items []interface{}) error) error {
	var err error
	items := make([]interface{}, 0, chunkSize)
	for item := range channel {
		items = append(items, item)
		if len(items) >= chunkSize {
			err = db.RunInTransaction(func(tx *pg.Tx) error {
				return merry.Wrap(handler(tx, items))
			})
			if err != nil {
				return merry.Wrap(err)
			}
			items = items[:0]
		}
	}
	if len(items) > 0 {
		err = db.RunInTransaction(func(tx *pg.Tx) error {
			return merry.Wrap(handler(tx, items))
		})
	}
	return merry.Wrap(err)
}
