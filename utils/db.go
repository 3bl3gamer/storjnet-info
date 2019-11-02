package utils

import (
	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/lib/pq"
)

func MakePGConnection() *pg.DB {
	db := pg.Connect(&pg.Options{User: "storjnet", Password: "storj", Database: "storjnet_db"})
	// db.OnQueryProcessed(func(event *pg.QueryProcessedEvent) {
	// 	query, err := event.FormattedQuery()
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	log.Printf("\033[36m%s \033[34m%s\033[39m", time.Since(event.StartTime), query)
	// })
	return db
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
