package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/keep94/consume2"
	"github.com/keep94/finances/fin"
	csqlite "github.com/keep94/finances/fin/categories/categoriesdb/for_sqlite"
	"github.com/keep94/finances/fin/checks"
	"github.com/keep94/finances/fin/findb"
	"github.com/keep94/finances/fin/findb/for_sqlite"
	"github.com/keep94/toolbox/date_util"
	"github.com/keep94/toolbox/db"
	"github.com/keep94/toolbox/db/sqlite3_db"
	_ "github.com/mattn/go-sqlite3"
)

var (
	fDb        string
	fAccount   string
	fStartDate string
	fEndDate   string
)

func main() {
	flag.Parse()
	if fDb == "" || fAccount == "" {
		fmt.Println("Need to specify db and account name")
		flag.Usage()
		os.Exit(1)
	}
	sdPtr, sderr := getDateRelaxed(fStartDate)
	edPtr, ederr := getDateRelaxed(fEndDate)
	if sderr != nil || ederr != nil {
		fmt.Println("Start and end date must be in yyyyMMdd format")
		flag.Usage()
		os.Exit(1)
	}

	// fDb
	rawdb, err := sql.Open("sqlite3", fDb)
	if err != nil {
		log.Fatal(err)
	}
	dbase := sqlite3_db.New(rawdb)
	defer dbase.Close()
	doer := sqlite3_db.NewDoer(dbase)
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
	pipeline := consume2.Identity[fin.EntryBalance]()
	if sdPtr != nil {
		pipeline = consume2.Join(
			pipeline,
			consume2.PTakeWhile(func(eb fin.EntryBalance) bool {
				return !eb.Date.Before(*sdPtr)
			}))
	}
	if edPtr != nil {
		pipeline = consume2.Join(
			pipeline,
			consume2.PFilter(func(eb fin.EntryBalance) bool {
				return eb.Date.Before(*edPtr)
			}))
	}
	err = doer.Do(func(t db.Transaction) error {
		return findb.EntriesByAccountId(
			t,
			store,
			accountDetail.Id(),
			&account,
			pipeline.Run(consume2.MaybeMap(
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
				})))
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

func normalizeYMDStr(dateStr string) string {
	if len(dateStr) == 4 {
		return dateStr + "0101"
	}
	if len(dateStr) == 6 {
		return dateStr + "01"
	}
	return dateStr
}

func getDateRelaxed(dateStr string) (*time.Time, error) {
	s := strings.TrimSpace(dateStr)
	if s == "" {
		return nil, nil
	}
	t, e := time.Parse(date_util.YMDFormat, normalizeYMDStr(s))
	if e != nil {
		return nil, e
	}
	return &t, nil
}

func init() {
	flag.StringVar(&fDb, "db", "", "Path to database file.")
	flag.StringVar(&fAccount, "account", "", "Name of account")
	flag.StringVar(
		&fStartDate, "sd", "", "Optional start date in yyyyMMdd format")
	flag.StringVar(
		&fEndDate, "ed", "", "Optional end date in yyyyMMdd format")
}
