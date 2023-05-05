// Package for_sqlite provides a sqlite implementation for storing processed
// QFX file fitIds.
package for_sqlite

import (
	"database/sql"

	"github.com/keep94/finances/fin/autoimport/qfx/qfxdb"
	"github.com/keep94/toolbox/db"
	"github.com/keep94/toolbox/db/sqlite3_db"
)

const (
	kSQLByAcctIdFitId     = "select acct_id from qfx_fitids where acct_id = ? and fit_id = ?"
	kSQLInsertAcctIdFitId = "insert into qfx_fitids (acct_id, fit_id) values (?, ?)"
)

// New creates sqlite implementation of qfxdb.Store interface
func New(db *sqlite3_db.Db) qfxdb.Store {
	return sqliteStore{db}
}

func add(tx *sql.Tx, accountId int64, fitIds qfxdb.FitIdSet) error {
	addStmt, err := tx.Prepare(kSQLInsertAcctIdFitId)
	if err != nil {
		return err
	}
	defer addStmt.Close()
	for fitId, ok := range fitIds {
		if ok {
			_, err := addStmt.Exec(accountId, fitId)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func findByAccountIdAndFitId(
	stmt *sql.Stmt, accountId int64, fitId string) (bool, error) {
	dbrows, err := stmt.Query(accountId, fitId)
	if err != nil {
		return false, err
	}
	defer dbrows.Close()
	found := dbrows.Next()
	return found, dbrows.Err()
}

func find(tx *sql.Tx, accountId int64, fitIds qfxdb.FitIdSet) (qfxdb.FitIdSet, error) {
	stmt, err := tx.Prepare(kSQLByAcctIdFitId)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	var result qfxdb.FitIdSet
	for fitId, ok := range fitIds {
		if ok {
			found, err := findByAccountIdAndFitId(stmt, accountId, fitId)
			if err != nil {
				return nil, err
			}
			if found {
				if result == nil {
					result = make(qfxdb.FitIdSet)
				}
				result[fitId] = true
			}
		}
	}
	return result, nil
}

type sqliteStore struct {
	db sqlite3_db.Doer
}

func (s sqliteStore) Add(
	t db.Transaction, accountId int64, fitIds qfxdb.FitIdSet) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return add(tx, accountId, fitIds)
	})
}

func (s sqliteStore) Find(
	t db.Transaction, accountId int64, fitIds qfxdb.FitIdSet) (found qfxdb.FitIdSet, err error) {
	err = sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) (err error) {
		found, err = find(tx, accountId, fitIds)
		return
	})
	return
}
