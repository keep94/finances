// Package for_sqlite stores types in categories package in a sqlite database.
package for_sqlite

import (
	"database/sql"

	"github.com/keep94/consume2"
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/categories"
	fsqlite "github.com/keep94/finances/fin/findb/for_sqlite"
	"github.com/keep94/toolbox/db/sqlite3_rw"
)

// CatDetails populates a CatDetailStore object from the database.
func catDetails(tx *sql.Tx) (
	cds categories.CatDetailStore, err error) {
	cdsb := categories.CatDetailStoreBuilder{}
	cdc := categories.CatDetailConsumer{Builder: &cdsb, Type: fin.ExpenseCat}
	if err = expenseCategories(tx, &cdc); err != nil {
		return
	}
	cdc.Type = fin.IncomeCat
	if err = incomeCategories(tx, &cdc); err != nil {
		return
	}
	adc := categories.AccountDetailConsumer{Builder: &cdsb}
	if err = fsqlite.ConnNew(tx).Accounts(nil, &adc); err != nil {
		return
	}
	cds = cdsb.Build()
	return
}

// accountStoreUpdater updates a sqlite database on behalf of a
// fin.CatDetailStore value.
type accountStoreUpdater struct {
	C *sql.Tx
}

func (u accountStoreUpdater) Add(name string) (newId int64, err error) {
	account := fin.Account{
		Name:   name,
		Active: true,
	}
	if err = fsqlite.ConnNew(u.C).AddAccount(nil, &account); err != nil {
		return
	}
	newId = account.Id
	return
}

func (u accountStoreUpdater) Update(id int64, newName string) error {
	store := fsqlite.ConnNew(u.C)
	var account fin.Account
	err := store.AccountById(nil, id, &account)
	if err != nil {
		return err
	}
	account.Name = newName
	account.Active = true
	return store.UpdateAccount(nil, &account)
}

func (u accountStoreUpdater) Remove(id int64) error {
	store := fsqlite.ConnNew(u.C)
	var account fin.Account
	err := store.AccountById(nil, id, &account)
	if err != nil {
		return err
	}
	account.Active = false
	return store.UpdateAccount(nil, &account)
}

// catDetailStoreUpdater updates a sqlite database on behalf of a
// fin.CatDetailStore value.
type catDetailStoreUpdater struct {
	C *sql.Tx
}

func (u catDetailStoreUpdater) Add(t fin.CatType, row *categories.CatDbRow) error {
	values, err := sqlite3_rw.InsertValues((&rawCatDbRow{}).init(row))
	if err != nil {
		return err
	}
	var result sql.Result
	if t == fin.ExpenseCat {
		result, err = u.C.Exec("insert into expense_categories (name, is_active, parent_id) values (?, ?, ?)", values...)
	} else if t == fin.IncomeCat {
		result, err = u.C.Exec("insert into income_categories (name, is_active, parent_id) values (?, ?, ?)", values...)
	} else {
		panic("t must be either ExpenseCat or IncomeCat")
	}
	if err != nil {
		return err
	}
	row.Id, err = result.LastInsertId()
	return err
}

func (u catDetailStoreUpdater) Update(t fin.CatType, row *categories.CatDbRow) error {
	values, err := sqlite3_rw.UpdateValues((&rawCatDbRow{}).init(row))
	if err != nil {
		return err
	}
	if t == fin.ExpenseCat {
		_, err := u.C.Exec("update expense_categories set name = ?, is_active = ?, parent_id = ? where id = ?", values...)
		return err
	} else if t == fin.IncomeCat {
		_, err := u.C.Exec("update income_categories set name = ?, is_active = ?, parent_id = ? where id = ?", values...)
		return err
	} else {
		panic("t must be either ExpenseCat or IncomeCat")
	}
}

func (u catDetailStoreUpdater) Remove(t fin.CatType, id int64) error {
	if t == fin.ExpenseCat {
		_, err := u.C.Exec("update expense_categories set is_active = 0 where id = ?", id)
		return err
	} else if t == fin.IncomeCat {
		_, err := u.C.Exec("update income_categories set is_active = 0 where id = ?", id)
		return err
	}
	return categories.NeedExpenseIncomeCategory
}

type rawCatDbRow struct {
	*categories.CatDbRow
	sqlite3_rw.SimpleRow
}

func (r *rawCatDbRow) init(bo *categories.CatDbRow) *rawCatDbRow {
	r.CatDbRow = bo
	return r
}

func (r *rawCatDbRow) Ptrs() []interface{} {
	return []interface{}{&r.Id, &r.Name, &r.Active, &r.ParentId}
}

func (r *rawCatDbRow) Values() []interface{} {
	return []interface{}{r.Name, r.Active, r.ParentId, r.Id}
}

func (r *rawCatDbRow) ValueRead() categories.CatDbRow {
	return *r.CatDbRow
}

func expenseCategories(
	tx *sql.Tx, consumer consume2.Consumer[categories.CatDbRow]) error {
	return sqlite3_rw.ReadMultiple[categories.CatDbRow](
		tx,
		(&rawCatDbRow{}).init(&categories.CatDbRow{}),
		consumer,
		"select id, name, is_active, parent_id from expense_categories")
}

func incomeCategories(
	tx *sql.Tx, consumer consume2.Consumer[categories.CatDbRow]) error {
	return sqlite3_rw.ReadMultiple[categories.CatDbRow](
		tx,
		(&rawCatDbRow{}).init(&categories.CatDbRow{}),
		consumer,
		"select id, name, is_active, parent_id from income_categories")
}
