package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/keep94/consume2"
	"github.com/keep94/finances/fin"
	csqlite "github.com/keep94/finances/fin/categories/categoriesdb/for_sqlite"
	"github.com/keep94/finances/fin/checks"
	"github.com/keep94/finances/fin/findb"
	"github.com/keep94/finances/fin/findb/for_sqlite"
	"github.com/keep94/gosqlite/sqlite"
	"github.com/keep94/toolbox/db"
	"github.com/keep94/toolbox/db/sqlite_db"
)

var (
	fDb      string
	fAccount string
)

func main() {
	flag.Parse()
	if fDb == "" || fAccount == "" {
		fmt.Println("Need to specify db and account name")
		flag.Usage()
		os.Exit(1)
	}

	// fDb
	conn, err := sqlite.Open(fDb)
	if err != nil {
		log.Fatal(err)
	}
	dbase := sqlite_db.New(conn)
	defer dbase.Close()
	doer := sqlite_db.NewDoer(dbase)
	cache := csqlite.New(dbase)
	store := for_sqlite.New(dbase)
	cds, _ := cache.Get(nil)

	accountDetail, ok := cds.AccountDetailByName(fAccount)
	if !ok {
		fmt.Printf("Unknown account: %s\n", fAccount)
		os.Exit(1)
	}
	var account fin.Account
	var checkNos []int
	err = doer.Do(func(t db.Transaction) error {
		return findb.EntriesByAccountId(
			t,
			store,
			accountDetail.Id(),
			&account,
			consume2.MaybeMap(
				consume2.AppendTo(&checkNos),
				func(entry fin.EntryBalance) (int, bool) {
					// It can't be a valid check if it is a credit
					if entry.Total() > 0 {
						return 0, false
					}
					result, err := strconv.Atoi(entry.CheckNo)
					if err != nil {
						return 0, false
					}
					return result, true
				}))
	})
	if err != nil {
		log.Fatal(err)
	}
	missing := checks.Missing(checkNos)
	if missing == nil {
		fmt.Println("No checks found in account.")
		return
	}
	fmt.Printf("First check: %d\n", missing.First)
	fmt.Printf("Last check: %d\n", missing.Last)
	fmt.Println("Missing checks:")
	for _, hole := range missing.Holes {
		if hole.First == hole.Last {
			fmt.Printf("  %d\n", hole.First)
		} else {
			fmt.Printf("  %d-%d\n", hole.First, hole.Last)
		}
	}
}

func init() {
	flag.StringVar(&fDb, "db", "", "Path to database file.")
	flag.StringVar(&fAccount, "account", "", "Name of account")
}
