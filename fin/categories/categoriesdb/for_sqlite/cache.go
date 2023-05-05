package for_sqlite

import (
	"database/sql"
	"sync"

	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/categories"
	"github.com/keep94/finances/fin/categories/categoriesdb"
	"github.com/keep94/toolbox/db"
	"github.com/keep94/toolbox/db/sqlite3_db"
)

func New(db *sqlite3_db.Db) *Cache {
	return &Cache{db: db}
}

func ReadOnlyWrapper(c *Cache) ReadOnlyCache {
	return ReadOnlyCache{cache: c}
}

type catDetailCache struct {
	mutex sync.Mutex
	data  categories.CatDetailStore
	valid bool
}

func (c *catDetailCache) DbGet(db *sqlite3_db.Db) (
	cds categories.CatDetailStore, err error) {
	cds, ok := c.getFromCache()
	if ok {
		return
	}
	err = db.Do(func(tx *sql.Tx) (err error) {
		cds, err = c.load(tx)
		return
	})
	return
}

func (c *catDetailCache) Get(tx *sql.Tx) (
	cds categories.CatDetailStore, err error) {
	cds, ok := c.getFromCache()
	if ok {
		return
	}
	return c.load(tx)
}

func (c *catDetailCache) Invalidate(tx *sql.Tx) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.valid = false
	return nil
}

func (c *catDetailCache) AccountAdd(tx *sql.Tx, name string) (
	cds categories.CatDetailStore, newId int64, err error) {
	if cds, err = catDetails(tx); err != nil {
		cds, _ = c.getFromCache()
		return
	}
	cds, newId, err = cds.AccountAdd(name, accountStoreUpdater{tx})
	c.save(cds)
	return
}

func (c *catDetailCache) AccountRename(
	tx *sql.Tx, id int64, name string) (
	cds categories.CatDetailStore, err error) {
	if cds, err = catDetails(tx); err != nil {
		cds, _ = c.getFromCache()
		return
	}
	cds, err = cds.AccountRename(id, name, accountStoreUpdater{tx})
	c.save(cds)
	return
}

func (c *catDetailCache) AccountRemove(
	tx *sql.Tx, id int64) (
	cds categories.CatDetailStore, err error) {
	if cds, err = catDetails(tx); err != nil {
		cds, _ = c.getFromCache()
		return
	}
	cds, err = cds.AccountRemove(id, accountStoreUpdater{tx})
	c.save(cds)
	return
}

func (c *catDetailCache) Add(tx *sql.Tx, name string) (
	cds categories.CatDetailStore, newId fin.Cat, err error) {
	if cds, err = catDetails(tx); err != nil {
		cds, _ = c.getFromCache()
		return
	}
	cds, newId, err = cds.Add(name, catDetailStoreUpdater{tx})
	c.save(cds)
	return
}

func (c *catDetailCache) Remove(tx *sql.Tx, id fin.Cat) (
	cds categories.CatDetailStore, err error) {
	if cds, err = catDetails(tx); err != nil {
		cds, _ = c.getFromCache()
		return
	}
	cds, err = cds.Remove(id, catDetailStoreUpdater{tx})
	c.save(cds)
	return
}

func (c *catDetailCache) Purge(tx *sql.Tx, cats fin.CatSet) error {
	expenseStmt, err := tx.Prepare("delete from expense_categories where id = ?")
	if err != nil {
		return err
	}
	defer expenseStmt.Close()
	incomeStmt, err := tx.Prepare("delete from income_categories where id = ?")
	if err != nil {
		return err
	}
	defer incomeStmt.Close()
	for cat, ok := range cats {
		if ok {
			if cat.Type == fin.ExpenseCat {
				if _, err := expenseStmt.Exec(cat.Id); err != nil {
					return err
				}
			} else if cat.Type == fin.IncomeCat {
				if _, err := incomeStmt.Exec(cat.Id); err != nil {
					return err
				}
			} else {
				return categories.NeedExpenseIncomeCategory
			}
		}
	}
	return c.Invalidate(tx)
}

func (c *catDetailCache) Rename(tx *sql.Tx, id fin.Cat, newName string) (
	cds categories.CatDetailStore, err error) {
	if cds, err = catDetails(tx); err != nil {
		cds, _ = c.getFromCache()
		return
	}
	cds, err = cds.Rename(id, newName, catDetailStoreUpdater{tx})
	c.save(cds)
	return
}

func (c *catDetailCache) save(cds categories.CatDetailStore) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.data = cds
	c.valid = true
}

func (c *catDetailCache) load(tx *sql.Tx) (
	cds categories.CatDetailStore, err error) {
	if cds, err = catDetails(tx); err != nil {
		return
	}
	c.save(cds)
	return
}

func (c *catDetailCache) getFromCache() (cds categories.CatDetailStore, ok bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if !c.valid {
		return
	}
	return c.data, true
}

type Cache struct {
	db *sqlite3_db.Db
	c  catDetailCache
}

func (c *Cache) AccountAdd(t db.Transaction, name string) (
	cds categories.CatDetailStore, newId int64, err error) {
	err = sqlite3_db.ToDoer(c.db, t).Do(func(tx *sql.Tx) (err error) {
		cds, newId, err = c.c.AccountAdd(tx, name)
		return
	})
	return
}

func (c *Cache) AccountRename(t db.Transaction, id int64, name string) (
	cds categories.CatDetailStore, err error) {
	err = sqlite3_db.ToDoer(c.db, t).Do(func(tx *sql.Tx) (err error) {
		cds, err = c.c.AccountRename(tx, id, name)
		return
	})
	return
}

func (c *Cache) AccountRemove(t db.Transaction, id int64) (
	cds categories.CatDetailStore, err error) {
	err = sqlite3_db.ToDoer(c.db, t).Do(func(tx *sql.Tx) (err error) {
		cds, err = c.c.AccountRemove(tx, id)
		return
	})
	return
}

func (c *Cache) Add(t db.Transaction, name string) (
	cds categories.CatDetailStore, newId fin.Cat, err error) {
	err = sqlite3_db.ToDoer(c.db, t).Do(func(tx *sql.Tx) (err error) {
		cds, newId, err = c.c.Add(tx, name)
		return
	})
	return
}

func (c *Cache) Get(t db.Transaction) (
	cds categories.CatDetailStore, err error) {
	if t != nil {
		err = sqlite3_db.ToDoer(c.db, t).Do(func(tx *sql.Tx) (err error) {
			cds, err = c.c.Get(tx)
			return
		})
		return
	}
	return c.c.DbGet(c.db)
}

func (c *Cache) Invalidate(t db.Transaction) error {
	return sqlite3_db.ToDoer(c.db, t).Do(func(tx *sql.Tx) error {
		return c.c.Invalidate(tx)
	})
}

func (c *Cache) Remove(t db.Transaction, id fin.Cat) (
	cds categories.CatDetailStore, err error) {
	err = sqlite3_db.ToDoer(c.db, t).Do(func(tx *sql.Tx) (err error) {
		cds, err = c.c.Remove(tx, id)
		return
	})
	return
}

func (c *Cache) Purge(t db.Transaction, cats fin.CatSet) error {
	return sqlite3_db.ToDoer(c.db, t).Do(func(tx *sql.Tx) error {
		return c.c.Purge(tx, cats)
	})
}

func (c *Cache) Rename(t db.Transaction, id fin.Cat, newName string) (
	cds categories.CatDetailStore, err error) {
	err = sqlite3_db.ToDoer(c.db, t).Do(func(tx *sql.Tx) (err error) {
		cds, err = c.c.Rename(tx, id, newName)
		return
	})
	return
}

// The writing methods of ReadOnlyCache merely return
// categoriesdb.NoPermission error along with the contents of the cache.
// If nothing is in the cache, they read from the database.
type ReadOnlyCache struct {
	categoriesdb.NoPermissionCache
	cache *Cache
}

func (c ReadOnlyCache) Get(t db.Transaction) (
	cds categories.CatDetailStore, err error) {
	return c.cache.Get(t)
}

func (c ReadOnlyCache) AccountAdd(t db.Transaction, name string) (
	cds categories.CatDetailStore, newId int64, err error) {
	cds, err = c.reportNoPermission(t)
	return
}

func (c ReadOnlyCache) AccountRename(t db.Transaction, id int64, name string) (
	cds categories.CatDetailStore, err error) {
	return c.reportNoPermission(t)
}

func (c ReadOnlyCache) AccountRemove(t db.Transaction, id int64) (
	cds categories.CatDetailStore, err error) {
	return c.reportNoPermission(t)
}

func (c ReadOnlyCache) Add(t db.Transaction, name string) (
	cds categories.CatDetailStore, newId fin.Cat, err error) {
	cds, err = c.reportNoPermission(t)
	return
}

func (c ReadOnlyCache) Remove(t db.Transaction, id fin.Cat) (
	cds categories.CatDetailStore, err error) {
	return c.reportNoPermission(t)
}

func (c ReadOnlyCache) Rename(
	t db.Transaction, id fin.Cat, newName string) (
	cds categories.CatDetailStore, err error) {
	return c.reportNoPermission(t)
}

func (c ReadOnlyCache) reportNoPermission(t db.Transaction) (
	cds categories.CatDetailStore, err error) {
	cds, _ = c.cache.Get(t)
	err = categoriesdb.NoPermission
	return
}
