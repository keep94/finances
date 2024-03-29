// Package for_sqlite stores types in fin package in a sqlite database.
package for_sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/keep94/consume2"
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/findb"
	"github.com/keep94/toolbox/db"
	"github.com/keep94/toolbox/db/sqlite3_db"
	"github.com/keep94/toolbox/db/sqlite3_rw"
	"github.com/keep94/toolbox/passwords"
)

const (
	kSQLEntryById                = "select id, date, name, desc, check_no, cats, payment, reviewed from entries where id = ?"
	kSQLEntriesPrefix            = "select id, date, name, desc, check_no, cats, payment, reviewed from entries"
	kSQLEntries                  = "select id, date, name, desc, check_no, cats, payment, reviewed from entries order by date desc, id desc"
	kSQLEntryOrderBy             = " order by date desc, id desc"
	kSQLInsertEntry              = "insert into entries (date, name, desc, check_no, cats, payment, reviewed) values (?, ?, ?, ?, ?, ?, ?)"
	kSQLUpdateEntry              = "update entries set date = ?, name = ?, desc = ?, check_no = ?, cats = ?, payment = ?, reviewed = ? where id = ?"
	kSQLDeleteEntryById          = "delete from entries where id = ?"
	kSQLRecurringEntryById       = "select id, date, name, desc, check_no, cats, payment, reviewed, count, unit, num_left, day_of_month from recurring_entries where id = ?"
	kSQLRecurringEntries         = "select id, date, name, desc, check_no, cats, payment, reviewed, count, unit, num_left, day_of_month from recurring_entries order by date, id"
	kSQLInsertRecurringEntry     = "insert into recurring_entries (date, name, desc, check_no, cats, payment, reviewed, count, unit, num_left, day_of_month) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	kSQLUpdateRecurringEntry     = "update recurring_entries set date = ?, name = ?, desc = ?, check_no = ?, cats = ?, payment = ?, reviewed = ?, count = ?, unit = ?, num_left = ?, day_of_month = ? where id = ?"
	kSQLDeleteRecurringEntryById = "delete from recurring_entries where id = ?"
	kSQLAccountById              = "select id, name, is_active, balance, reconciled, b_count, r_count, import_sd from accounts where id = ?"
	kSQLAccounts                 = "select id, name, is_active, balance, reconciled, b_count, r_count, import_sd from accounts"
	kSQLActiveAccounts           = "select id, name, is_active, balance, reconciled, b_count, r_count, import_sd from accounts where is_active = 1 order by name"
	kSQLInsertAccount            = "insert into accounts (name, is_active, balance, reconciled, b_count, r_count, import_sd) values (?, ?, ?, ?, ?, ?, ?)"
	kSQLUpdateAccountImportSD    = "update accounts set import_sd = ? where id = ?"
	kSQLUpdateAccount            = "update accounts set name = ?, is_active = ?, balance = ?, reconciled = ?, b_count = ?, r_count = ?, import_sd = ? where id = ?"
	kSQLRemoveAccount            = "delete from accounts where id = ?"
	kSQLUserById                 = "select id, name, go_password, permission, last_login from users where id = ?"
	kSQLUsers                    = "select id, name, go_password, permission, last_login from users order by name"
	kSQLRemoveUserByName         = "delete from users where name = ?"
	kSQLUserByName               = "select id, name, go_password, permission, last_login from users where name = ?"
	kSQLInsertUser               = "insert into users (name, go_password, permission, last_login) values (?, ?, ?, ?)"
	kSQLUpdateUser               = "update users set name = ?, go_password = ?, permission = ?, last_login = ? where id = ?"
	kSQLAllocationsByYear        = "select expense_id, amount from allocations where year = ?"
	kSQLAddAllocation            = "insert into allocations (year, expense_id, amount) values (?, ?, ?)"
	kSQLRemoveAllocation         = "delete from allocations where year = ? and expense_id = ?"
)

func New(db *sqlite3_db.Db) Store {
	return Store{db}
}

func ConnNew(tx *sql.Tx) Store {
	return Store{sqlite3_db.NewSqlite3Doer(tx)}
}

func ReadOnlyWrapper(store Store) ReadOnlyStore {
	return ReadOnlyStore{store: store}
}

func entries(tx *sql.Tx, options *findb.EntryListOptions, consumer consume2.Consumer[fin.Entry]) error {
	var sql string
	if options != nil {
		where_clauses := make([]string, 3)
		where_clause_count := 0
		if options.Start != nil {
			where_clauses[where_clause_count] = "date >= ?"
			where_clause_count++
		}
		if options.End != nil {
			where_clauses[where_clause_count] = "date < ?"
			where_clause_count++
		}
		if options.Unreviewed {
			where_clauses[where_clause_count] = "reviewed != 1"
			where_clause_count++
		}
		if where_clause_count > 0 {
			sql = kSQLEntriesPrefix + " where " + strings.Join(where_clauses[:where_clause_count], " and ") + kSQLEntryOrderBy
		} else {
			sql = kSQLEntriesPrefix + kSQLEntryOrderBy
		}
	} else {
		sql = kSQLEntriesPrefix + kSQLEntryOrderBy
	}
	sql_params := make([]interface{}, 0, 2)
	if options != nil {
		if options.Start != nil {
			sql_params = append(
				sql_params, sqlite3_db.DateToString(*options.Start))
		}
		if options.End != nil {
			sql_params = append(
				sql_params, sqlite3_db.DateToString(*options.End))
		}
	}
	dbrows, err := tx.Query(sql, sql_params...)
	if err != nil {
		return err
	}
	defer dbrows.Close()
	if options != nil && options.Unreviewed {
		return sqlite3_rw.ReadRowsWithEtag[fin.Entry](
			(&rawEntry{}).init(&fin.Entry{}),
			dbrows,
			consumer)
	}
	return sqlite3_rw.ReadRows[fin.Entry](
		(&rawEntry{}).init(&fin.Entry{}),
		dbrows,
		consumer)
}

func entryById(tx *sql.Tx, id int64, entry *fin.Entry) error {
	stmt, err := tx.Prepare(kSQLEntryById)
	if err != nil {
		return err
	}
	defer stmt.Close()
	return _entryById(stmt, (&rawEntry{}).init(entry), id)
}

func _entryById(stmt *sql.Stmt, r *rawEntry, id int64) error {
	dbrows, err := stmt.Query(id)
	if err != nil {
		return err
	}
	defer dbrows.Close()
	return sqlite3_rw.FirstOnly(r, dbrows, findb.NoSuchId)
}

func doEntryChanges(tx *sql.Tx, changes *findb.EntryChanges) error {
	row := (&rawEntry{}).init(&fin.Entry{})
	var err error
	var deltas fin.AccountDeltas = make(map[int64]*fin.AccountDelta)
	var getStmt, addStmt, deleteStmt, updateStmt *sql.Stmt
	if len(changes.Updates) > 0 || len(changes.Deletes) > 0 {
		getStmt, err = tx.Prepare(kSQLEntryById)
		if err != nil {
			return err
		}
		defer getStmt.Close()
	}
	if len(changes.Adds) > 0 {
		addStmt, err = tx.Prepare(kSQLInsertEntry)
		if err != nil {
			return err
		}
		defer addStmt.Close()
	}
	if len(changes.Deletes) > 0 {
		deleteStmt, err = tx.Prepare(kSQLDeleteEntryById)
		if err != nil {
			return err
		}
		defer deleteStmt.Close()
	}
	if len(changes.Updates) > 0 {
		updateStmt, err = tx.Prepare(kSQLUpdateEntry)
		if err != nil {
			return err
		}
		defer updateStmt.Close()
	}
	for _, id := range changes.Deletes {
		err = _entryById(getStmt, row, id)
		if err == findb.NoSuchId {
			continue
		}
		if err != nil {
			return err
		}
		deltas.Exclude(&row.CatPayment)
		_, err = deleteStmt.Exec(id)
		if err != nil {
			return err
		}
	}
	for id, update := range changes.Updates {
		err = _entryById(getStmt, row, id)
		if err == findb.NoSuchId {
			continue
		}
		if err != nil {
			return err
		}
		concurrent_update_detected := false
		if changes.Etags != nil {
			expected_etag, ok := changes.Etags[id]
			if !ok {
				panic("Etags field present, but does not contain etag for all updated entries.")
			}
			if expected_etag != row.Etag {
				concurrent_update_detected = true
			}
		}
		old_cat_payment := row.CatPayment
		if !update(row.Entry) {
			continue
		}
		if concurrent_update_detected {
			return findb.ConcurrentUpdate
		}
		deltas.Exclude(&old_cat_payment)
		deltas.Include(&row.CatPayment)
		row.Entry.Id = id
		var updateValues []interface{}
		if updateValues, err = sqlite3_rw.UpdateValues(row); err != nil {
			return err
		}
		_, err = updateStmt.Exec(updateValues...)
		if err != nil {
			return err
		}
	}
	for _, entry := range changes.Adds {
		row.init(entry)
		deltas.Include(&entry.CatPayment)
		err = addEntry(addStmt, row)
		if err != nil {
			return err
		}
	}
	return recordAccountDeltas(tx, deltas)
}

func activeAccounts(tx *sql.Tx) (accounts []*fin.Account, err error) {
	err = sqlite3_rw.ReadMultiple[fin.Account](
		tx,
		(&rawAccount{}).init(&fin.Account{}),
		consume2.AppendPtrsTo(&accounts),
		kSQLActiveAccounts)
	return
}

func updateAccountImportSD(tx *sql.Tx, acctId int64, date time.Time) error {
	_, err := tx.Exec(kSQLUpdateAccountImportSD, sqlite3_db.DateToString(date), acctId)
	return err
}

func addEntry(stmt *sql.Stmt, r *rawEntry) error {
	values, err := sqlite3_rw.InsertValues(r)
	if err != nil {
		return err
	}
	result, err := stmt.Exec(values...)
	if err != nil {
		return err
	}
	r.Id, err = result.LastInsertId()
	return err
}

func recordAccountDeltas(tx *sql.Tx, deltas fin.AccountDeltas) error {
	for id, delta := range deltas {
		_, err := tx.Exec("update accounts set balance = balance + ?, reconciled = reconciled + ?, b_count = b_count + ?, r_count = r_count + ? where id = ?", delta.Balance, delta.RBalance, delta.Count, delta.RCount, id)
		if err != nil {
			return err
		}
	}
	return nil
}

type rawEntry struct {
	*fin.Entry
	dateStr string
	cat     string
	payment string
	status  int
}

func (r *rawEntry) init(bo *fin.Entry) *rawEntry {
	r.Entry = bo
	return r
}

func (r *rawEntry) Ptrs() []interface{} {
	return []interface{}{&r.Id, &r.dateStr, &r.Name, &r.Desc, &r.CheckNo, &r.cat, &r.payment, &r.status}
}

func (r *rawEntry) Values() []interface{} {
	return []interface{}{r.dateStr, r.Name, r.Desc, r.CheckNo, r.cat, r.payment, r.status, r.Id}
}

func (r *rawEntry) SetEtag(etag uint64) {
	r.Etag = etag
}

func (r *rawEntry) ValueRead() fin.Entry {
	return *r.Entry
}

func (r *rawEntry) Unmarshall() error {
	var err error
	if r.Entry.Date, err = sqlite3_db.StringToDate(r.dateStr); err != nil {
		return err
	}
	r.Status = fin.ReviewStatus(r.status)
	return r.Entry.Unmarshall(r, unmarshall)
}

func (r *rawEntry) Marshall() error {
	r.dateStr = sqlite3_db.DateToString(r.Date)
	r.status = int(r.Status)
	r.Entry.Marshall(marshall, r)
	return nil
}

type rawRecurringEntry struct {
	*fin.RecurringEntry
	re   rawEntry
	unit int
}

func (r *rawRecurringEntry) init(bo *fin.RecurringEntry) *rawRecurringEntry {
	r.RecurringEntry = bo
	r.re.init(&bo.Entry)
	return r
}

func (r *rawRecurringEntry) Ptrs() []interface{} {
	return []interface{}{&r.Id, &r.re.dateStr, &r.Name, &r.Desc, &r.CheckNo, &r.re.cat, &r.re.payment, &r.re.status, &r.Period.Count, &r.unit, &r.NumLeft, &r.Period.DayOfMonth}
}

func (r *rawRecurringEntry) Values() []interface{} {
	return []interface{}{r.re.dateStr, r.Name, r.Desc, r.CheckNo, r.re.cat, r.re.payment, r.re.status, r.Period.Count, r.unit, r.NumLeft, r.Period.DayOfMonth, r.Id}
}

func (r *rawRecurringEntry) SetEtag(etag uint64) {
	r.Etag = etag
}

func (r *rawRecurringEntry) ValueRead() fin.RecurringEntry {
	return *r.RecurringEntry
}

func (r *rawRecurringEntry) Unmarshall() (err error) {
	if err = r.re.Unmarshall(); err != nil {
		return
	}
	var valid bool
	if r.Period.Unit, valid = fin.ToRecurringUnit(r.unit); !valid {
		err = errors.New("Invalid recurring unit found in database.")
	}
	return
}

func (r *rawRecurringEntry) Marshall() (err error) {
	if err = r.re.Marshall(); err != nil {
		return
	}
	r.unit = r.Period.Unit.ToInt()
	return
}

type rawAccount struct {
	*fin.Account
	importSDStr string
}

func (r *rawAccount) init(bo *fin.Account) *rawAccount {
	r.Account = bo
	return r
}

func (r *rawAccount) Ptrs() []interface{} {
	return []interface{}{&r.Id, &r.Name, &r.Active, &r.Balance, &r.RBalance, &r.Count, &r.RCount, &r.importSDStr}
}

func (r *rawAccount) Values() []interface{} {
	return []interface{}{r.Name, r.Active, r.Balance, r.RBalance, r.Count, r.RCount, r.importSDStr, r.Id}
}

func (r *rawAccount) ValueRead() fin.Account {
	return *r.Account
}

func (r *rawAccount) Unmarshall() error {
	r.Account.ImportSD, _ = sqlite3_db.StringToDate(r.importSDStr)
	return nil
}

func (r *rawAccount) Marshall() error {
	r.importSDStr = sqlite3_db.DateToString(r.ImportSD)
	return nil
}

type rawUser struct {
	*fin.User
	rawPassword   string
	rawPermission int
	rawLastLogin  int64
}

func (r *rawUser) init(bo *fin.User) *rawUser {
	r.User = bo
	return r
}

func (r *rawUser) Ptrs() []interface{} {
	return []interface{}{&r.Id, &r.Name, &r.rawPassword, &r.rawPermission, &r.rawLastLogin}
}

func (r *rawUser) Values() []interface{} {
	return []interface{}{r.Name, r.rawPassword, r.rawPermission, r.rawLastLogin, r.Id}
}

func (r *rawUser) ValueRead() fin.User {
	return *r.User
}

func (r *rawUser) Unmarshall() error {
	r.Password = passwords.Password(r.rawPassword)
	// Defaults to fin.NonePermission if the raw permission is not recognized
	r.Permission, _ = fin.ToPermission(r.rawPermission)
	if r.rawLastLogin == 0 {
		r.LastLogin = time.Time{}
	} else {
		r.LastLogin = time.Unix(r.rawLastLogin, 0).UTC()
	}
	return nil
}

func (r *rawUser) Marshall() error {
	r.rawPassword = string(r.Password)
	r.rawPermission = r.Permission.ToInt()
	if r.LastLogin.IsZero() {
		r.rawLastLogin = 0
	} else {
		r.rawLastLogin = r.LastLogin.Unix()
	}
	return nil
}

func unmarshall(ptr interface{}, cr *[]fin.CatRec, id *int64, reconciled *bool) error {
	p := ptr.(*rawEntry)
	var parts []string
	if p.cat != "" {
		parts = strings.Split(p.cat, "|")
	}
	partLen := len(parts)
	if partLen%3 != 0 {
		return errors.New(fmt.Sprintf("for_sqlite: Category string invalid: %s", p.cat))
	}
	if partLen != 0 {
		(*cr) = make([]fin.CatRec, partLen/3)
	} else {
		(*cr) = nil
	}
	for i := range *cr {
		a, err := strconv.ParseInt(parts[3*i+1], 10, 64)
		if err != nil {
			return err
		}
		r, err := strconv.ParseInt(parts[3*i+2], 10, 0)
		if err != nil {
			return err
		}
		if r > 0 {
			(*cr)[i] = fin.CatRec{Amount: a, Reconciled: true}
		} else {
			(*cr)[i] = fin.CatRec{Amount: a, Reconciled: false}
		}
		(*cr)[i].Cat, err = fin.CatFromString(parts[3*i])
		if err != nil {
			return err
		}
	}
	parts = strings.SplitN(p.payment, "|", 2)
	partLen = len(parts)
	if partLen < 2 {
		return errors.New(fmt.Sprintf("for_sqlite: Payment string invalid: %s", p.payment))
	}
	r, err := strconv.ParseInt(parts[1], 10, 0)
	if err != nil {
		return err
	}
	if r > 0 {
		*reconciled = true
	} else {
		*reconciled = false
	}
	pc, err := fin.CatFromString(parts[0])
	if err != nil {
		return err
	}
	*id = pc.Id
	return nil
}

func marshall(cr []fin.CatRec, id int64, reconciled bool, ptr interface{}) {
	p := ptr.(*rawEntry)
	catStrs := make([]string, 3*len(cr))
	for i := range cr {
		catStrs[3*i] = cr[i].Cat.ToString()
		catStrs[3*i+1] = strconv.FormatInt(cr[i].Amount, 10)
		if cr[i].Reconciled {
			catStrs[3*i+2] = "1"
		} else {
			catStrs[3*i+2] = "0"
		}
	}
	paymentStrs := make([]string, 2)
	pc := fin.Cat{Id: id, Type: fin.AccountCat}
	paymentStrs[0] = pc.ToString()
	if reconciled {
		paymentStrs[1] = "1"
	} else {
		paymentStrs[1] = "0"
	}
	p.cat = strings.Join(catStrs, "|")
	p.payment = strings.Join(paymentStrs, "|")
}

type Store struct {
	db sqlite3_db.Doer
}

func (s Store) AccountById(
	t db.Transaction, acctId int64, account *fin.Account) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return sqlite3_rw.ReadSingle(
			tx,
			(&rawAccount{}).init(account),
			findb.NoSuchId,
			kSQLAccountById,
			acctId)
	})
}

func (s Store) Accounts(
	t db.Transaction, consumer consume2.Consumer[fin.Account]) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return sqlite3_rw.ReadMultiple[fin.Account](
			tx,
			(&rawAccount{}).init(&fin.Account{}),
			consumer,
			kSQLAccounts)
	})
}

func (s Store) ActiveAccounts(t db.Transaction) (
	accounts []*fin.Account, err error) {
	err = sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) (err error) {
		accounts, err = activeAccounts(tx)
		return
	})
	return
}

func (s Store) AddAccount(t db.Transaction, account *fin.Account) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return sqlite3_rw.AddRow(
			tx, (&rawAccount{}).init(account), &account.Id, kSQLInsertAccount)
	})
}

func (s Store) DoEntryChanges(
	t db.Transaction, changes *findb.EntryChanges) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return doEntryChanges(tx, changes)
	})
}

func (s Store) Entries(
	t db.Transaction,
	options *findb.EntryListOptions,
	consumer consume2.Consumer[fin.Entry]) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return entries(tx, options, consumer)
	})
}

func (s Store) EntryById(
	t db.Transaction, id int64, entry *fin.Entry) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return entryById(tx, id, entry)
	})
}

func (s Store) UpdateAccountImportSD(
	t db.Transaction, acctId int64, date time.Time) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return updateAccountImportSD(tx, acctId, date)
	})
}

func (s Store) UpdateAccount(
	t db.Transaction, account *fin.Account) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return sqlite3_rw.UpdateRow(
			tx, (&rawAccount{}).init(account), kSQLUpdateAccount)
	})
}

func (s Store) RemoveAccount(
	t db.Transaction, id int64) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		_, err := tx.Exec(kSQLRemoveAccount, id)
		return err
	})
}

func (s Store) AddUser(t db.Transaction, user *fin.User) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return sqlite3_rw.AddRow(
			tx, (&rawUser{}).init(user), &user.Id, kSQLInsertUser)
	})
}

func (s Store) RemoveUserByName(t db.Transaction, name string) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		_, err := tx.Exec(kSQLRemoveUserByName, name)
		return err
	})
}

func (s Store) UpdateUser(t db.Transaction, user *fin.User) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return sqlite3_rw.UpdateRow(
			tx, (&rawUser{}).init(user), kSQLUpdateUser)
	})
}

func (s Store) UserById(
	t db.Transaction, id int64, user *fin.User) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return sqlite3_rw.ReadSingle(
			tx,
			(&rawUser{}).init(user),
			findb.NoSuchId,
			kSQLUserById,
			id)
	})
}

func (s Store) UserByName(
	t db.Transaction, name string, user *fin.User) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return sqlite3_rw.ReadSingle(
			tx,
			(&rawUser{}).init(user),
			findb.NoSuchId,
			kSQLUserByName,
			name)
	})
}

func (s Store) Users(
	t db.Transaction, consumer consume2.Consumer[fin.User]) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return sqlite3_rw.ReadMultiple[fin.User](
			tx,
			(&rawUser{}).init(&fin.User{}),
			consumer,
			kSQLUsers)
	})
}

func (s Store) AddRecurringEntry(
	t db.Transaction, entry *fin.RecurringEntry) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return sqlite3_rw.AddRow(
			tx,
			(&rawRecurringEntry{}).init(entry),
			&entry.Id,
			kSQLInsertRecurringEntry)
	})
}

func (s Store) UpdateRecurringEntry(
	t db.Transaction, entry *fin.RecurringEntry) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return sqlite3_rw.UpdateRow(
			tx, (&rawRecurringEntry{}).init(entry), kSQLUpdateRecurringEntry)
	})
}

func (s Store) RecurringEntryById(
	t db.Transaction, id int64, entry *fin.RecurringEntry) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return sqlite3_rw.ReadSingle(
			tx,
			(&rawRecurringEntry{}).init(entry),
			findb.NoSuchId,
			kSQLRecurringEntryById,
			id)
	})
}

func (s Store) RecurringEntries(
	t db.Transaction, consumer consume2.Consumer[fin.RecurringEntry]) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		return sqlite3_rw.ReadMultiple[fin.RecurringEntry](
			tx,
			(&rawRecurringEntry{}).init(&fin.RecurringEntry{}),
			consumer,
			kSQLRecurringEntries)
	})
}

func (s Store) RemoveRecurringEntryById(t db.Transaction, id int64) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		_, err := tx.Exec(kSQLDeleteRecurringEntryById, id)
		return err
	})
}

func (s Store) AllocationsByYear(t db.Transaction, year int64) (
	result map[int64]int64, err error) {
	err = sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) (err error) {
		result, err = allocationsByYear(tx, year)
		return
	})
	return
}

func allocationsByYear(tx *sql.Tx, year int64) (map[int64]int64, error) {
	dbrows, err := tx.Query(kSQLAllocationsByYear, year)
	if err != nil {
		return nil, err
	}
	defer dbrows.Close()
	result := make(map[int64]int64)
	for dbrows.Next() {
		var expenseId, amount int64
		if err := dbrows.Scan(&expenseId, &amount); err != nil {
			return nil, err
		}
		result[expenseId] = amount
	}
	return result, nil
}

func (s Store) RemoveAllocation(
	t db.Transaction, year, expenseId int64) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		_, err := tx.Exec(kSQLRemoveAllocation, year, expenseId)
		return err
	})
}

func (s Store) AddAllocation(
	t db.Transaction, year, expenseId, amount int64) error {
	return sqlite3_db.ToDoer(s.db, t).Do(func(tx *sql.Tx) error {
		_, err := tx.Exec(kSQLAddAllocation, year, expenseId, amount)
		return err
	})
}

type ReadOnlyStore struct {
	findb.NoPermissionStore
	store Store
}

func (s ReadOnlyStore) AccountById(
	t db.Transaction, acctId int64, account *fin.Account) error {
	return s.store.AccountById(t, acctId, account)
}

func (s ReadOnlyStore) Accounts(
	t db.Transaction, consumer consume2.Consumer[fin.Account]) error {
	return s.store.Accounts(t, consumer)
}

func (s ReadOnlyStore) ActiveAccounts(t db.Transaction) (
	accounts []*fin.Account, err error) {
	return s.store.ActiveAccounts(t)
}

func (s ReadOnlyStore) Entries(
	t db.Transaction,
	options *findb.EntryListOptions,
	consumer consume2.Consumer[fin.Entry]) error {
	return s.store.Entries(t, options, consumer)
}

func (s ReadOnlyStore) EntryById(
	t db.Transaction, id int64, entry *fin.Entry) error {
	return s.store.EntryById(t, id, entry)
}

func (s ReadOnlyStore) UserById(
	t db.Transaction, id int64, user *fin.User) error {
	return s.store.UserById(t, id, user)
}

func (s ReadOnlyStore) UserByName(
	t db.Transaction, name string, user *fin.User) error {
	return s.store.UserByName(t, name, user)
}

func (s ReadOnlyStore) Users(
	t db.Transaction, consumer consume2.Consumer[fin.User]) error {
	return s.store.Users(t, consumer)
}

func (s ReadOnlyStore) RecurringEntryById(
	t db.Transaction, id int64, entry *fin.RecurringEntry) error {
	return s.store.RecurringEntryById(t, id, entry)
}

func (s ReadOnlyStore) RecurringEntries(
	t db.Transaction, consumer consume2.Consumer[fin.RecurringEntry]) error {
	return s.store.RecurringEntries(t, consumer)
}

func (s ReadOnlyStore) AllocationsByYear(t db.Transaction, year int64) (
	map[int64]int64, error) {
	return s.store.AllocationsByYear(t, year)
}
