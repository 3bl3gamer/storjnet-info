package utils

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/abh/geoip"
	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/lib/pq"
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

func MakeGeoIPConnection() (*geoip.GeoIP, error) {
	gdb, err := geoip.Open("/usr/share/GeoIP/GeoIPCity.dat")
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return gdb, nil
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
