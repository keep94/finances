package csv_test

import (
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/autoimport"
	"github.com/keep94/finances/fin/autoimport/csv"
	"github.com/keep94/finances/fin/autoimport/qfx/qfxdb"
	"github.com/keep94/toolbox/date_util"
	"github.com/keep94/toolbox/db"
	"github.com/stretchr/testify/assert"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const kChaseCsv = `
Card,Transaction Date,Post Date,Description,Category,Type,Amount,Memo
5824,10/12/2023,10/13/2023,LOZANO SUNNYVALE CARWASH,Automotive,Sale,-42.99,
5824,10/12/2023,10/13/2023,APPLE.COM/US,Shopping,Sale,-181.41,
5824,10/12/2023,10/13/2023,SUNNYVALE GAS,Gas,Sale,-83.87,
`

const kPaypalCsv = `
Date, Time, Time Zone, Name, Type, Status, Amount, Receipt ID, Balance,
"12/6/2015","07:59:04","PST","TrackR, Inc","Express Checkout Payment Sent","Completed","-87.00","","0.00",
"12/6/2015","07:59:04","PST","Bank Account","Add Funds from a Bank Account","Completed","87.00","","87.00",
"9/5/2015","09:15:47","PST","Starbucks Coffee Company","Express Checkout Payment Sent","Completed","-48.10","","0.00",
"9/5/2015","09:15:47","PST","Bank Account","Add Funds from a Bank Account","Completed","48.10","","48.10",
"09/03/2015","17:55:28","PST","Disney Online","Express Checkout Payment Sent","Completed","-46.41","","0.00",
"09/03/2015","17:55:28","PST","Bank Account","Add Funds from a Bank Account","Completed","46.41","","46.41",
"9/2/2015","09:27:09","PST","Disney Online","Express Checkout Payment Sent","Completed","-18.43","","0.00",
"9/2/2015","09:27:09","PST","Bank Account","Add Funds from a Bank Account","Completed","18.43","","18.43",
`

const kMissingNameCsv = `
Date, Time, Time Zone, Name, Type, Status, Amount, Receipt ID, Balance,
"12/6/2015","07:59:04","PST","TrackR, Inc","Express Checkout Payment Sent","Completed","-87.00","","0.00",
"9/5/2015","09:15:47","PST","Starbucks Coffee Company","Express Checkout Payment Sent","Completed","-48.10","","0.00",
"9/2/2015","09:27:09","PST","","Express Checkout Payment Sent","Completed","-18.43","","0.00",
"9/2/2015","09:27:09","PST","Bank Account","Add Funds from a Bank Account","Completed","18.43","","18.43",
`

func TestReadBadCsvFile(t *testing.T) {
	r := strings.NewReader("A bad file\nNo CSV things in here\n")
	var loader autoimport.Loader
	loader = csv.CsvLoader{make(storeType)}
	_, err := loader.Load(3, "", r, date_util.YMD(2012, 11, 14))
	if err == nil {
		t.Error("Expected error")
	}
}

func TestReadCsvWithEntryMissingName(t *testing.T) {
	var loader autoimport.Loader
	loader = csv.CsvLoader{make(storeType)}
	r := strings.NewReader(kMissingNameCsv)
	_, err := loader.Load(3, "", r, date_util.YMD(2015, 9, 3))
	if err != nil {
		t.Error("Expected no error")
	}
	r = strings.NewReader(kMissingNameCsv)
	_, err = loader.Load(3, "", r, date_util.YMD(2015, 9, 2))
	if err == nil {
		t.Error("Expected error reading entry with missing name")
	}
}

func TestReadPaypalCsv(t *testing.T) {
	r := strings.NewReader(kPaypalCsv)
	var loader autoimport.Loader
	loader = csv.CsvLoader{make(storeType)}
	batch, err := loader.Load(3, "", r, date_util.YMD(2015, 9, 3))
	if err != nil {
		t.Errorf("Got error %v", err)
		return
	}
	entries := batch.Entries()
	expectedEntries := []*fin.Entry{
		{
			Date:       date_util.YMD(2015, 12, 6),
			Name:       "TrackR, Inc",
			CatPayment: fin.NewCatPayment(fin.Expense, 8700, true, 3)},
		{
			Date:       date_util.YMD(2015, 9, 5),
			Name:       "Starbucks Coffee Company",
			CatPayment: fin.NewCatPayment(fin.Expense, 4810, true, 3)},
		{
			Date:       date_util.YMD(2015, 9, 3),
			Name:       "Disney Online",
			CatPayment: fin.NewCatPayment(fin.Expense, 4641, true, 3)}}
	if !reflect.DeepEqual(expectedEntries, entries) {
		t.Errorf("Expected %v, got %v", expectedEntries, entries)
	}
}

func TestReadChaseCsv(t *testing.T) {
	r := strings.NewReader(kChaseCsv)
	var loader autoimport.Loader
	loader = csv.CsvLoader{make(storeType)}
	batch, err := loader.Load(3, "", r, date_util.YMD(2023, 10, 12))
	if err != nil {
		t.Errorf("Got error %v", err)
		return
	}
	entries := batch.Entries()
	expectedEntries := []*fin.Entry{
		{
			Date:       date_util.YMD(2023, 10, 12),
			Name:       "LOZANO SUNNYVALE CARWASH",
			CatPayment: fin.NewCatPayment(fin.Expense, 4299, true, 3)},
		{
			Date:       date_util.YMD(2023, 10, 12),
			Name:       "APPLE.COM/US",
			CatPayment: fin.NewCatPayment(fin.Expense, 18141, true, 3)},
		{
			Date:       date_util.YMD(2023, 10, 12),
			Name:       "SUNNYVALE GAS",
			CatPayment: fin.NewCatPayment(fin.Expense, 8387, true, 3)}}
	assert.Equal(t, expectedEntries, entries)
}

func TestMarkProcessed(t *testing.T) {
	r := strings.NewReader(kPaypalCsv)
	fitIdStore := make(storeType)
	loader := csv.CsvLoader{fitIdStore}
	batch, err := loader.Load(3, "", r, date_util.YMD(2015, 9, 3))
	if err != nil {
		t.Errorf("Got error %v", err)
		return
	}
	batch.MarkProcessed(nil)
	r = strings.NewReader(kPaypalCsv)
	newBatch, err := loader.Load(3, "", r, date_util.YMD(2015, 9, 2))
	if err != nil {
		t.Errorf("Got error %v", err)
		return
	}
	newBatch, _ = newBatch.SkipProcessed(nil)
	if output := len(newBatch.Entries()); output != 1 {
		t.Errorf("Expected 1, got %v", output)
	}
	fitIds := fitIdStore[3]
	assert.NotEmpty(t, fitIds)
	for id := range fitIds {
		assert.True(t, validateId(id), "Bad FitId: "+id)
	}
}

func validateId(id string) bool {
	idx := strings.Index(id, ":")
	if idx != 8 {
		return false
	}
	dateint, err := strconv.ParseInt(id[:idx], 10, 64)
	if err != nil {
		return false
	}
	if dateint < 20150000 || dateint >= 20160000 {
		return false
	}
	checksum, err := strconv.ParseUint(id[idx+1:], 10, 64)
	if err != nil {
		return false
	}
	return checksum != 0
}

type storeType map[int64]map[string]bool

func (s storeType) Add(t db.Transaction, accountId int64, fitIds qfxdb.FitIdSet) error {
	if s[accountId] == nil {
		s[accountId] = make(map[string]bool)
	}
	for fitId, ok := range fitIds {
		if ok {
			s[accountId][fitId] = true
		}
	}
	return nil
}

func (s storeType) Find(t db.Transaction, accountId int64, fitIds qfxdb.FitIdSet) (qfxdb.FitIdSet, error) {
	var result qfxdb.FitIdSet
	for fitId, ok := range fitIds {
		if ok {
			if s[accountId][fitId] {
				if result == nil {
					result = make(qfxdb.FitIdSet)
				}
				result[fitId] = true
			}
		}
	}
	return result, nil
}
