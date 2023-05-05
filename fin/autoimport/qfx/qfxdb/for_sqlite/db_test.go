package for_sqlite

import (
	"database/sql"
	"testing"

	"github.com/keep94/finances/fin/autoimport/qfx/qfxdb/fixture"
	"github.com/keep94/finances/fin/findb/sqlite_setup"
	"github.com/keep94/toolbox/db/sqlite3_db"
	_ "github.com/mattn/go-sqlite3"
)

func TestFind(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).Find(t)
}

func newFixture(db *sqlite3_db.Db) *fixture.Fixture {
	return &fixture.Fixture{Store: New(db), Doer: sqlite3_db.NewDoer(db)}
}

func closeDb(t *testing.T, db *sqlite3_db.Db) {
	if err := db.Close(); err != nil {
		t.Errorf("Error closing database: %v", err)
	}
}

func openDb(t *testing.T) *sqlite3_db.Db {
	rawdb, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Error opening database: %v", err)
	}
	db := sqlite3_db.New(rawdb)
	err = db.Do(sqlite_setup.SetUpTables)
	if err != nil {
		t.Fatalf("Error creating tables: %v", err)
	}
	return db
}
