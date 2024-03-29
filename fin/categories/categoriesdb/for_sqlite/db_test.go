package for_sqlite

import (
	"database/sql"
	"testing"

	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/categories"
	"github.com/keep94/finances/fin/categories/categoriesdb/fixture"
	fsqlite "github.com/keep94/finances/fin/findb/for_sqlite"
	"github.com/keep94/finances/fin/findb/sqlite_setup"
	"github.com/keep94/toolbox/db"
	"github.com/keep94/toolbox/db/sqlite3_db"
	_ "github.com/mattn/go-sqlite3"
)

func TestCatDetails(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CatDetails(t)
}

func TestCatDetailGoodAdd(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CatDetailGoodAdd(t)
}

func TestCatDetailsBadAdds(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CatDetailsBadAdds(t)
}

func TestCatDetailsRename(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CatDetailsRename(t)
}

func TestCatDetailsRename2(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CatDetailsRename2(t)
}

func TestCatDetailsRenameSame(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CatDetailsRenameSame(t)
}

func TestCatDetailsRenameBad(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CatDetailsRenameBad(t)
}

func TestRemoveCatDetail(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).RemoveCatDetail(t)
}

func TestRemoveCatDetail2(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).RemoveCatDetail2(t)
}

func TestRemoveCatDetailMissing(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).RemoveCatDetailMissing(t)
}

func TestRemoveCatDetailError(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).RemoveCatDetailError(t)
}

func TestCacheGet(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheGet(t, New(db))
}

func TestCatDetailInvalidate(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CatDetailInvalidate(t, New(db))
}

func TestCacheAdd(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheAdd(t, New(db))
}

func TestCacheAddError(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheAddError(t, New(db))
}

func TestCacheRename(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheRename(t, New(db))
}

func TestCacheRenameError(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheRenameError(t, New(db))
}

func TestCacheRemove(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheRemove(t, New(db))
}

func TestCacheRemoveError(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheRemoveError(t, New(db))
}

func TestCacheAccountAdd(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheAccountAdd(t, New(db))
}

func TestCacheAccountAddError(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheAccountAddError(t, New(db))
}

func TestCacheAccountAddMalformed(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheAccountAddMalformed(t, New(db))
}

func TestCacheAccountRename(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheAccountRename(t, New(db))
}

func TestCacheAccountRenameSame(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheAccountRenameSame(t, New(db))
}

func TestCacheAccountRenameError(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheAccountRenameError(t, New(db))
}

func TestCacheAccountRenameError2(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheAccountRenameError2(t, New(db))
}

func TestCacheAccountRenameMalformed(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheAccountRenameMalformed(t, New(db))
}

func TestCacheAccountRemove(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheAccountRemove(t, New(db))
}

func TestCacheAccountRemoveError(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CacheAccountRemoveError(t, New(db))
}

func TestCachePurge(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	newFixture(db).CachePurge(t, New(db))
}

func newFixture(db *sqlite3_db.Db) *fixture.Fixture {
	return &fixture.Fixture{
		Store: fsqlite.New(db),
		Doer:  sqlite3_db.NewDoer(db),
		Db:    dbstubb{db}}
}

type dbstubb struct {
	db *sqlite3_db.Db
}

func (d dbstubb) Read(t db.Transaction) (
	cds categories.CatDetailStore, err error) {
	err = sqlite3_db.ToDoer(d.db, t).Do(func(tx *sql.Tx) (err error) {
		cds, err = catDetails(tx)
		return
	})
	return
}

func (d dbstubb) Add(
	t db.Transaction, cds categories.CatDetailStore, name string) (
	newStore categories.CatDetailStore, newId fin.Cat, err error) {
	err = sqlite3_db.ToDoer(d.db, t).Do(func(tx *sql.Tx) (err error) {
		newStore, newId, err = cds.Add(name, catDetailStoreUpdater{C: tx})
		return
	})
	return
}

func (d dbstubb) Rename(
	t db.Transaction, cds categories.CatDetailStore, id fin.Cat, name string) (
	newStore categories.CatDetailStore, err error) {
	err = sqlite3_db.ToDoer(d.db, t).Do(func(tx *sql.Tx) (err error) {
		newStore, err = cds.Rename(id, name, catDetailStoreUpdater{C: tx})
		return
	})
	return
}

func (d dbstubb) Remove(
	t db.Transaction, cds categories.CatDetailStore, id fin.Cat) (
	newStore categories.CatDetailStore, err error) {
	err = sqlite3_db.ToDoer(d.db, t).Do(func(tx *sql.Tx) (err error) {
		newStore, err = cds.Remove(id, catDetailStoreUpdater{C: tx})
		return
	})
	return
}

func openDb(t *testing.T) *sqlite3_db.Db {
	rawdb, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Error opening database: %v", err)
	}
	dbase := sqlite3_db.New(rawdb)
	err = dbase.Do(sqlite_setup.SetUpTables)
	if err != nil {
		t.Fatalf("Error creating tables: %v", err)
	}
	return dbase
}

func closeDb(t *testing.T, db *sqlite3_db.Db) {
	if err := db.Close(); err != nil {
		t.Errorf("Error closing database: %v", err)
	}
}
