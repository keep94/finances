// Package csv provides processing of csv files
package csv

import (
	gocsv "encoding/csv"
	"errors"
	"fmt"
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/autoimport"
	"github.com/keep94/finances/fin/autoimport/qfx"
	"github.com/keep94/finances/fin/autoimport/qfx/qfxdb"
	"github.com/keep94/toolbox/date_util"
	"hash/fnv"
	"io"
	"strconv"
	"time"
)

// CsvLoader implements the autoimport.Loader interface for csv files.
type CsvLoader struct {
	// Store stores which transactions have already been processed.
	Store qfxdb.Store
}

func (c CsvLoader) Load(
	accountId int64,
	bankAccountId string,
	r io.Reader,
	startDate time.Time) (autoimport.Batch, error) {
	reader := gocsv.NewReader(r)
	line, err := reader.Read()
	if err != nil {
		return nil, err
	}
	parser := fromHeader(line)
	if parser == nil {
		return nil, errors.New("Unrecognized csv header")
	}
	var result []*qfx.QfxEntry
	for line, err = reader.Read(); err == nil; line, err = reader.Read() {
		var qentry qfx.QfxEntry
		var ok bool
		ok, err = parser.ParseLine(line, accountId, &qentry.Entry)
		if err != nil {
			return nil, err
		}
		if !ok || qentry.Date.Before(startDate) {
			continue
		}
		qentry.FitId, err = generateFitId(
			qentry.Date, fitIdColumns(line, parser.FitIdColumnIndexes()))
		if err != nil {
			return nil, err
		}
		err = qentry.Check()
		if err != nil {
			return nil, err
		}
		result = append(result, &qentry)
	}
	if err != io.EOF {
		return nil, err
	}
	return &qfx.QfxBatch{Store: c.Store, AccountId: accountId, QfxEntries: result}, nil
}

// csvParser is responsible for parsing csv files.
type csvParser interface {

	// ParseLine parses one line of csv file. It writes line to entry.
	// It returns true if successful, false and the error on error, or false
	// and no error if line should be skipped.
	ParseLine(line []string, accountId int64, entry *fin.Entry) (ok bool, err error)

	// FitIdColumnIndexes returns the column indexes to use when computing
	// FITID. Order is important.
	FitIdColumnIndexes() []int
}

type nativeCsvParser struct {
}

func (nativeCsvParser) ParseLine(
	line []string, accountId int64, entry *fin.Entry) (ok bool, err error) {
	entry.Date, err = time.Parse("1/2/2006", line[0])
	if err != nil {
		return
	}
	entry.CheckNo = line[1]
	entry.Name = line[2]
	entry.Desc = line[3]
	var amt int64
	amt, err = fin.ParseUSD(line[4])
	if err != nil {
		return
	}
	entry.CatPayment = fin.NewCatPayment(fin.Expense, -amt, true, accountId)
	ok = true
	return
}

func (nativeCsvParser) FitIdColumnIndexes() []int {
	return []int{0, 1, 2, 3, 4}
}

type paypalCsvParser struct {
}

func (paypalCsvParser) ParseLine(
	line []string, accountId int64, entry *fin.Entry) (ok bool, err error) {
	entry.Date, err = time.Parse("1/2/2006", line[0])
	if err != nil {
		return
	}
	entry.Name = line[3]
	if entry.Name == "Bank Account" {
		return
	}
	var amt int64
	amt, err = fin.ParseUSD(line[6])
	if err != nil {
		return
	}
	entry.CatPayment = fin.NewCatPayment(fin.Expense, -amt, true, accountId)
	ok = true
	return
}

func (paypalCsvParser) FitIdColumnIndexes() []int {
	return []int{0, 3, 6}
}

type chaseCsvParser struct {
	DateIndex   int
	NameIndex   int
	AmountIndex int
}

func (c *chaseCsvParser) ParseLine(
	line []string, accountId int64, entry *fin.Entry) (ok bool, err error) {
	entry.Date, err = time.Parse("1/2/2006", line[c.DateIndex])
	if err != nil {
		return
	}
	entry.Name = line[c.NameIndex]
	var amt int64
	amt, err = fin.ParseUSD(line[c.AmountIndex])
	if err != nil {
		return
	}
	entry.CatPayment = fin.NewCatPayment(fin.Expense, -amt, true, accountId)
	ok = true
	return
}

func (c *chaseCsvParser) FitIdColumnIndexes() []int {
	return []int{c.DateIndex, c.NameIndex, c.AmountIndex}
}

func fromHeader(line []string) csvParser {
	if len(line) == 10 && line[0] == "Date" && line[3] == " Name" && line[6] == " Amount" {
		return paypalCsvParser{}
	}
	if len(line) == 5 && line[0] == "Date" && line[1] == "CheckNo" && line[2] == "Name" && line[3] == "Desc" && line[4] == "Amount" {
		return nativeCsvParser{}
	}
	if len(line) == 8 && line[1] == "Transaction Date" && line[3] == "Description" && line[6] == "Amount" {
		return &chaseCsvParser{
			DateIndex:   1,
			NameIndex:   3,
			AmountIndex: 6,
		}
	}
	if len(line) == 7 && line[0] == "Transaction Date" && line[2] == "Description" && line[5] == "Amount" {
		return &chaseCsvParser{
			DateIndex:   0,
			NameIndex:   2,
			AmountIndex: 5,
		}
	}
	return nil
}

func fitIdColumns(line []string, columns []int) []string {
	result := make([]string, len(columns))
	for i := range columns {
		result[i] = line[columns[i]]
	}
	return result
}

func generateFitId(date time.Time, line []string) (string, error) {
	h := fnv.New64a()
	s := fmt.Sprintf("%v", line)
	_, err := h.Write(([]byte)(s))
	if err != nil {
		return "", err
	}
	dateStr := date.Format(date_util.YMDFormat)
	return dateStr + ":" + strconv.FormatUint(h.Sum64(), 10), nil
}
