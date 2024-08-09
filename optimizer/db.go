package optimizer

import (
	"regexp"
	"storjnet/utils"
	"strings"
	"time"

	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v9"
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
)

func OptimizeDB() error {
	db := utils.MakePGConnection()

	if err := updateDatePartitions(db, "user_nodes_history", 6); err != nil {
		return merry.Wrap(err)
	}

	log.Info().Msg("pausing a bit")
	time.Sleep(3 * time.Second)

	for _, name := range []string{"user_nodes_history__current" /*, "node_daily_stats"*/} {
		log.Info().Str("name", name).Msg("vacuuming")
		if err := vacuumIfHaveEnoughSpace(db, name); err != nil {
			return merry.Wrap(err)
		}
	}

	log.Info().Msg("done.")
	return nil
}

func updateDatePartitions(db *pg.DB, parentTableName string, partitionDurationMonths int) error {
	tableName := func(startDate, endDate time.Time) string {
		return parentTableName + "__" + startDate.Format("2006_01_02") + "__" + endDate.Format("2006_01_02")
	}

	var allPartitionTableNames []string
	_, err := db.Query(&allPartitionTableNames, `
		SELECT
			child.relname AS name
		FROM pg_inherits
			JOIN pg_class parent ON pg_inherits.inhparent = parent.oid
			JOIN pg_class child  ON pg_inherits.inhrelid  = child.oid
		WHERE parent.relname = ?`, parentTableName)
	if err != nil {
		return merry.Wrap(err)
	}

	archiveTableNameRe := regexp.MustCompile(`^.*?__(\d{4}_\d\d_\d\d)__(\d{4}_\d\d_\d\d)$`)

	var currentTableName string
	var lastArchiveTable *ArchiveTable
	for _, name := range allPartitionTableNames {
		if strings.HasSuffix(name, "__current") {
			if currentTableName == "" {
				currentTableName = name
			} else {
				return merry.Errorf("multiple __current partition tables: %s and %s", currentTableName, name)
			}
		} else {
			match := archiveTableNameRe.FindStringSubmatch(name)
			if match == nil {
				return merry.Errorf("invalid partition table name: %s", name)
			}
			startDateStr := match[1]
			endDateStr := match[2]

			startDate, err := time.Parse("2006_01_02", startDateStr)
			if err != nil {
				return merry.Errorf("invalid date (%s) in partition table name: %s", startDateStr, name)
			}
			endDate, err := time.Parse("2006_01_02", endDateStr)
			if err != nil {
				return merry.Errorf("invalid date (%s) in partition table name: %s", endDateStr, name)
			}

			if lastArchiveTable == nil || endDate.After(lastArchiveTable.EndDate) {
				lastArchiveTable = &ArchiveTable{Name: name, StartDate: startDate, EndDate: endDate}
			}
		}
	}

	if currentTableName == "" {
		return merry.Errorf("no __current table among %s's partition tables: %v", parentTableName, allPartitionTableNames)
	}

	archiveEndOffset := time.Hour //durtion since midnight UTC after which nothing should update yesterday records
	archiveEndDate := time.Now().In(time.UTC).Add(-archiveEndOffset).Truncate(24 * time.Hour)
	log.Info().Str("date", archiveEndDate.String()).Str("name", parentTableName).Msg("going to archive records before")

	var minCurDate time.Time
	_, err = db.QueryOne(&minCurDate, `SELECT min(date) FROM `+currentTableName)
	if err != nil {
		return merry.Wrap(err)
	}

	if !minCurDate.IsZero() && minCurDate.Before(archiveEndDate) {
		log.Info().
			Str("old_date", minCurDate.Format("2006-01-02")).
			Msg("current partition has old enough records, going to transfer them to previous partition")

		// lastArchivePartitionJustCreated := false
		err := db.RunInTransaction(func(tx *pg.Tx) error {
			var lastArchiveTableIsAttached bool
			if lastArchiveTable == nil || lastArchiveTable.IsFullFor(partitionDurationMonths) {
				var startDate time.Time
				if lastArchiveTable == nil {
					startDate = truncMonthDown(minCurDate, partitionDurationMonths)
				} else {
					startDate = lastArchiveTable.EndDate
				}
				endDate := startDate

				newTableName := tableName(startDate, endDate)

				log.Info().Str("name", newTableName).Msg("creating new archive partition")

				_, err := tx.Exec(`CREATE TABLE storjnet.` + newTableName + ` (LIKE storjnet.` + currentTableName + ` INCLUDING ALL)`)
				if err != nil {
					return merry.Wrap(err)
				}

				lastArchiveTable = &ArchiveTable{Name: newTableName, StartDate: startDate, EndDate: endDate}
				lastArchiveTableIsAttached = false
				// lastArchivePartitionJustCreated = true
			} else {
				log.Info().Str("name", lastArchiveTable.Name).Msg("using existing archive partition")
				lastArchiveTableIsAttached = true
			}

			newEndDate := dateMin(archiveEndDate, lastArchiveTable.MaxEndDate(partitionDurationMonths))
			newArchTableName := tableName(lastArchiveTable.StartDate, newEndDate)

			log.Info().
				Str("from", lastArchiveTable.EndDate.Format("2006-01-02")).
				Str("to", newEndDate.Format("2006-01-02")).
				Msg("will move archive partition end")

			log.Info().Str("name", currentTableName).Msg("counting rows to move")
			var rowsCount int64
			_, err := tx.QueryOne(&rowsCount,
				`SELECT count(*) FROM storjnet.`+currentTableName+` WHERE date >= ? AND date < ?`,
				lastArchiveTable.StartDate, newEndDate)
			if err != nil {
				return merry.Wrap(err)
			}

			log.Info().Str("to", newArchTableName).Msg("renaming archive partition")
			_, err = tx.Exec(`ALTER TABLE storjnet.` + lastArchiveTable.Name + ` RENAME TO ` + newArchTableName + ``)
			if err != nil {
				return merry.Wrap(err)
			}

			log.Info().Str("name", parentTableName).Msg("EXCLUSIVE LOCK")
			_, err = tx.Exec(`LOCK TABLE storjnet.` + parentTableName + ` IN ACCESS EXCLUSIVE MODE`)
			if err != nil {
				return merry.Wrap(err)
			}

			if lastArchiveTableIsAttached {
				log.Info().Str("name", newArchTableName).Msg("detaching partition")
				_, err := tx.Exec(`ALTER TABLE storjnet.` + parentTableName + ` DETACH PARTITION storjnet.` + newArchTableName + ``)
				if err != nil {
					return merry.Wrap(err)
				}
			}

			log.Info().Int64("count", rowsCount).Msg("moving rows from current partition to archive")
			_, err = tx.Exec(`
				INSERT INTO storjnet.`+newArchTableName+`
					SELECT * FROM storjnet.`+currentTableName+` WHERE date >= ? AND date < ?`,
				lastArchiveTable.StartDate, newEndDate)
			if err != nil {
				return merry.Wrap(err)
			}

			log.Info().Msg("removing copied rows and reattaching")
			_, err = tx.Exec(`
				DELETE FROM storjnet.`+parentTableName+` WHERE date >= ? AND date < ?;
				ALTER TABLE storjnet.`+parentTableName+` ATTACH PARTITION storjnet.`+newArchTableName+` FOR VALUES FROM (?) TO (?);`,
				lastArchiveTable.StartDate, newEndDate,
				lastArchiveTable.StartDate.Format("2006-01-02"), newEndDate.Format("2006-01-02"))
			if err != nil {
				return merry.Wrap(err)
			}

			lastArchiveTable.Name = newArchTableName
			lastArchiveTable.EndDate = newEndDate

			// return merry.New("test")
			return nil
		})
		if err != nil {
			return merry.Wrap(err)
		}

		log.Info().Msg("commited")

		// locks parent table, disabled for now
		// if !lastArchivePartitionJustCreated {
		// 	log.Info().Str("name", lastArchiveTable.Name).Msg("vacuuming")
		// 	if err := vacuumIfHaveEnoughSpace(db, lastArchiveTable.Name); err != nil {
		// 		return merry.Wrap(err)
		// 	}
		// }
	} else {
		log.Info().Msg("nothing to archive")
	}

	log.Info().Str("name", parentTableName).Msg("done with partitions")
	return nil
}

type ArchiveTable struct {
	Name      string
	StartDate time.Time
	EndDate   time.Time
}

func (t ArchiveTable) MaxEndDate(partitionDurationMonths int) time.Time {
	return truncMonthDown(t.StartDate, partitionDurationMonths).AddDate(0, partitionDurationMonths, 0)
}

func (t ArchiveTable) IsFullFor(partitionDurationMonths int) bool {
	return !t.EndDate.Before(t.MaxEndDate(partitionDurationMonths))
}

func truncMonthDown(d time.Time, months int) time.Time {
	month := (int(d.Month()-1)/months)*months + 1
	return time.Date(d.Year(), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
}

func dateMin(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func vacuumIfHaveEnoughSpace(db *pg.DB, tableName string) error {
	var stat unix.Statfs_t
	if err := unix.Statfs("/var/lib/postgres", &stat); err != nil {
		return merry.Wrap(err)
	}
	free := int64(stat.Bavail) * stat.Bsize

	var tableSize int64
	_, err := db.QueryOne(&tableSize, `SELECT pg_total_relation_size(quote_ident(?))`, tableName)
	if err != nil {
		return merry.Wrap(err)
	}

	if free < tableSize*12/10+1024*1024*1024 {
		log.Warn().Int64("free", free).Int64("table_size", tableSize).Msg("too little free space")
		return nil
	}

	_, err = db.Exec(`VACUUM FULL ` + tableName)
	if err != nil {
		return merry.Wrap(err)
	}
	return nil
}
