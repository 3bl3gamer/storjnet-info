package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-pg/migrations"
	"github.com/go-pg/pg"
)

const usageText = `This program runs command on the db. Supported commands are:
  - up - runs all available migrations.
  - down - reverts last migration.
  - reset - reverts all migrations.
  - version - prints current db version.
  - set_version [version] - sets db version without running migrations.
Usage:
  go run *.go [args] <command>
`

func usage() {
	fmt.Printf(usageText)
	flag.PrintDefaults()
	os.Exit(2)
}

func execSome(db migrations.DB, queries ...string) error {
	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

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

func (d dbLogger) BeforeQuery(event *pg.QueryEvent) {
	event.Data["StartTime"] = time.Now()
}

func (d dbLogger) AfterQuery(event *pg.QueryEvent) {
	query, err := event.FormattedQuery()
	if err != nil {
		panic(err)
	}
	log.Printf("\033[36m%s\n\033[34m%s\033[39m", time.Since(event.Data["StartTime"].(time.Time)), align(query))
}

func main() {
	flag.Usage = usage
	dbname := flag.String("dbname", "storjinfo_db", "database name")
	flag.Parse()

	db := pg.Connect(&pg.Options{User: "storjinfo", Password: "storjinfo", Database: *dbname})
	db.AddQueryHook(dbLogger{})

	migrations.SetTableName("storjinfo.gopg_migrations")

	oldVersion, newVersion, err := migrations.Run(db, flag.Args()...)
	if err != nil {
		panic(err)
	}
	fmt.Printf("\033[32mMigrated\033[39m: %d -> %d\n", oldVersion, newVersion)
}