package for_sqlite

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/keep94/finances/fin/findb/fixture"
	"github.com/keep94/finances/fin/findb/sqlite_setup"
	"github.com/keep94/toolbox/db/sqlite3_db"
	_ "github.com/mattn/go-sqlite3"
)

var (
	changeError = errors.New("for_sqlite: Error while changing.")
)

func TestAccountUpdates(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).AccountUpdates(t, New(db))
}

func TestSaveAndLoadEntry(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).SaveAndLoadEntry(t, New(db))
}

func TestUpdateEntry(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).UpdateEntry(t, New(db))
}

func TestUpdateEntrySkipped(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).UpdateEntrySkipped(t, New(db))
}

func TestListEntries(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).ListEntries(t, New(db))
}

func TestDeleteEntries(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).DeleteEntries(t, New(db))
}

func TestListEntriesEmptyOptions(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).ListEntriesEmptyOptions(t, New(db))
}

func TestListEntriesDateRange(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).ListEntriesDateRange(t, New(db))
}

func TestListEntriesDateRangeAndUnreviewed(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).ListEntriesDateRangeAndUnreviewed(t, New(db))
}

func TestListEntriesJustStartDate(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).ListEntriesJustStartDate(t, New(db))
}

func TestListEntriesJustEndDate(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).ListEntriesJustEndDate(t, New(db))
}

func TestListEntriesUnreviewed(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).ListEntriesUnreviewed(t, New(db))
}

func TestEntriesByAccountId(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).EntriesByAccountId(t, New(db))
}

func TestEntriesByAccountIdNilPtr(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).EntriesByAccountIdNilPtr(t, New(db))
}

func TestUnreconciledEntries(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).UnreconciledEntries(t, New(db))
}

func TestUnreconciledEntriesNoAccount(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).UnreconciledEntriesNoAccount(t, New(db))
}

func TestConcurrentUpdateDetection(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).ConcurrentUpdateDetection(t, New(db))
}

func TestConcurrentUpdateSkipped(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).ConcurrentUpdateSkipped(t, New(db))
}

func TestApplyRecurringEntries(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).ApplyRecurringEntries(t, New(db))
}

func TestActiveAccounts(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).ActiveAccounts(t, New(db))
}

func TestUpdateAccountImportSD(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).UpdateAccountImportSD(t, New(db))
}

func TestUpdateAccount(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).UpdateAccount(t, New(db))
}

func TestRemoveAccount(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newEntryAccountFixture(db).RemoveAccount(t, New(db))
}

func TestUserById(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	fixture.UserById(t, New(db))
}

func TestUserByName(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	fixture.UserByName(t, New(db))
}

func TestUsers(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	fixture.Users(t, New(db))
}

func TestLoginUser(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	fixture.LoginUser(t, sqlite3_db.NewDoer(db), New(db))
}

func TestRemoveUserByName(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	fixture.RemoveUserByName(t, New(db))
}

func TestNoUserByName(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	fixture.NoUserByName(t, New(db))
}

func TestUpdateUser(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	fixture.UpdateUser(t, New(db))
}

func TestAllocations(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	fixture.Allocations(t, New(db))
}

func newEntryAccountFixture(db *sqlite3_db.Db) fixture.EntryAccountFixture {
	return fixture.EntryAccountFixture{Doer: sqlite3_db.NewDoer(db)}
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
