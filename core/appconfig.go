package core

import (
	"github.com/ansel1/merry"
	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

type DBTx interface {
	QueryOne(model, query interface{}, params ...interface{}) (orm.Result, error)
	Exec(query interface{}, params ...interface{}) (orm.Result, error)
}

func appConfigVal(db DBTx, key string, val interface{}, forUpdate bool) error {
	sql := "SELECT value FROM appconfig WHERE key = ?"
	if forUpdate {
		sql += " FOR UPDATE"
	}
	_, err := db.QueryOne(val, sql, key)
	return err
}

func AppConfigInt64Slice(db DBTx, key string, forUpdate bool) ([]int64, error) {
	var res struct{ Value []int64 }
	err := appConfigVal(db, key, &res, forUpdate)
	if err == pg.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, merry.Wrap(err)
	}
	return res.Value, nil
}

func AppConfigSet(db DBTx, key string, value interface{}) error {
	_, err := db.Exec(`
		INSERT INTO appconfig (key, value) VALUES (?,?)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, value)
	return merry.Wrap(err)
}
