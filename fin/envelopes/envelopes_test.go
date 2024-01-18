package envelopes

import (
	"errors"
	"testing"

	"github.com/keep94/consume2"
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/categories"
	"github.com/keep94/finances/fin/findb"
	"github.com/keep94/toolbox/date_util"
	"github.com/keep94/toolbox/db"
	"github.com/stretchr/testify/assert"
)

const (
	kNonNil = "nonnil"
)

var (
	kError = errors.New("An error")
)

func TestEnvelope(t *testing.T) {
	e := Envelope{Allocated: 6000, Spent: 2000}
	assert.Equal(t, int64(4000), e.Remaining())
	assert.Equal(t, int64(500), e.Progress())
}

func TestEnvelopeNoAlloc(t *testing.T) {
	e := Envelope{Spent: 2000}
	assert.Equal(t, int64(-2000), e.Remaining())
	assert.Equal(t, int64(0), e.Progress())
}

func TestEnvelopeNegativeSpend(t *testing.T) {
	e := Envelope{Allocated: 6000, Spent: -2000}
	assert.Equal(t, int64(100), e.Progress())
}

func TestEnvelopes(t *testing.T) {
	var es Envelopes
	es.add(Envelope{Allocated: 6000, Spent: 4000})
	es.add(Envelope{Allocated: 2000, Spent: 1000})
	assert.Equal(t, 2, es.Len())
	assert.Equal(t, int64(8000), es.TotalAllocated())
	assert.Equal(t, int64(5000), es.TotalSpent())
	assert.Equal(t, int64(3000), es.TotalRemaining())
	assert.Equal(t, int64(850), es.TotalProgress())
}

func TestEnvelopesNegativeAlloc(t *testing.T) {
	var es Envelopes
	es.add(Envelope{Allocated: -1000, Spent: 2000})
	assert.Equal(t, int64(0), es.TotalProgress())
}

func TestEnvelopesNegativeSpend(t *testing.T) {
	var es Envelopes
	es.add(Envelope{Allocated: 5000, Spent: -2000})
	assert.Equal(t, int64(100), es.TotalProgress())
}

func TestEnvelopesEmpty(t *testing.T) {
	var es Envelopes
	assert.Equal(t, int64(0), es.TotalAllocated())
	assert.Equal(t, int64(0), es.TotalSpent())
	assert.Equal(t, int64(0), es.TotalRemaining())
	assert.Equal(t, int64(0), es.TotalProgress())
}

func TestSummary(t *testing.T) {
	var es Envelopes
	es.add(Envelope{Allocated: 3000, Spent: 5000})
	es.add(Envelope{Allocated: 9000, Spent: 2000})
	summary := &Summary{Envelopes: es, TotalSpent: 8000}
	assert.Equal(t, int64(1000), summary.UncategorizedSpend())
	assert.Equal(t, int64(900), summary.TotalProgress())
}

func TestSummaryByYear(t *testing.T) {
	var fs fakeStore
	fs.addEntryWithCatPayment(
		fin.NewCatPayment(fin.NewCat("0:3"), 3500, false, 1))
	fs.addEntryWithCatPayment(
		fin.NewCatPayment(fin.NewCat("0:4"), 10000, false, 1))
	fs.addEntryWithCatPayment(
		fin.NewCatPayment(fin.NewCat("0:2"), 4500, false, 1))
	fs.addEntryWithCatPayment(
		fin.NewCatPayment(fin.NewCat("0:1"), 50000, false, 1))
	fs.addEntryWithCatPayment(
		fin.NewCatPayment(fin.NewCat("0:1"), 30000, false, 1))
	fs.addEntryWithCatPayment(
		fin.NewCatPayment(fin.NewCat("0:5"), 5000, false, 1))
	fs.setAllocation(1, 100000)
	fs.setAllocation(2, 45000)
	summary, err := SummaryByYear(kNonNil, &fs, createCds(), 2024)
	assert.NoError(t, err)
	var expectedEnvelopes Envelopes
	expectedEnvelopes.add(Envelope{
		ExpenseId: 2,
		Name:      "expense:car",
		Allocated: 45000,
		Spent:     18000})
	expectedEnvelopes.add(Envelope{
		ExpenseId: 1,
		Name:      "expense:house",
		Allocated: 100000,
		Spent:     80000})
	expectedSummary := &Summary{
		Envelopes: expectedEnvelopes, TotalSpent: 103000}
	assert.Equal(t, expectedSummary, summary)
	options := fs.OptionsFromLastCall
	assert.Equal(t, date_util.YMD(2024, 1, 1), *options.Start)
	assert.Equal(t, date_util.YMD(2025, 1, 1), *options.End)
	assert.False(t, options.Unreviewed)
}

func TestSummaryByYearNone(t *testing.T) {
	var fs fakeStore
	summary, err := SummaryByYear(kNonNil, &fs, createCds(), 2024)
	assert.NoError(t, err)
	assert.Zero(t, *summary)
}

func TestSummaryByYearEntriesError(t *testing.T) {
	var fs fakeStore
	fs.EntriesErrorToReturn = kError
	_, err := SummaryByYear(kNonNil, &fs, createCds(), 2024)
	assert.Error(t, err)
}

func TestSummaryByYearAllocationsError(t *testing.T) {
	var fs fakeStore
	fs.AllocationsErrorToReturn = kError
	_, err := SummaryByYear(kNonNil, &fs, createCds(), 2024)
	assert.Error(t, err)
}

func TestSummaryByYearPanicOnNilTransaction(t *testing.T) {
	var fs fakeStore
	assert.Panics(t, func() {
		SummaryByYear(nil, &fs, createCds(), 2024)
	})
}

type fakeEntriesStore struct {
	OptionsFromLastCall  *findb.EntryListOptions
	EntriesToReturn      []fin.Entry
	EntriesErrorToReturn error
}

func (f *fakeEntriesStore) Entries(
	t db.Transaction,
	options *findb.EntryListOptions,
	consumer consume2.Consumer[fin.Entry]) error {
	f.OptionsFromLastCall = copyOptions(options)
	if f.EntriesErrorToReturn != nil {
		return f.EntriesErrorToReturn
	}
	consume2.FromSlice(f.EntriesToReturn, consumer)
	return nil
}

func copyOptions(options *findb.EntryListOptions) *findb.EntryListOptions {
	if options == nil {
		return nil
	}
	result := *options
	return &result
}

func (f *fakeEntriesStore) addEntryWithCatPayment(cp fin.CatPayment) {
	f.EntriesToReturn = append(f.EntriesToReturn, fin.Entry{CatPayment: cp})
}

type fakeAllocationsStore struct {
	YearFromLastCall         int64
	AllocationsToReturn      map[int64]int64
	AllocationsErrorToReturn error
}

func (f *fakeAllocationsStore) AllocationsByYear(
	t db.Transaction, year int64) (map[int64]int64, error) {
	f.YearFromLastCall = year
	if f.AllocationsErrorToReturn != nil {
		return nil, f.AllocationsErrorToReturn
	}
	result := make(map[int64]int64)
	for k, v := range f.AllocationsToReturn {
		result[k] = v
	}
	return result, nil
}

func (f *fakeAllocationsStore) setAllocation(id, amt int64) {
	if f.AllocationsToReturn == nil {
		f.AllocationsToReturn = make(map[int64]int64)
	}
	f.AllocationsToReturn[id] = amt
}

type fakeStore struct {
	fakeEntriesStore
	fakeAllocationsStore
}

func createCds() categories.CatDetailStore {
	var builder categories.CatDetailStoreBuilder
	builder.AddAccount(&fin.Account{Id: 1, Name: "Checking"})
	builder.AddCatDbRow(
		fin.ExpenseCat,
		&categories.CatDbRow{Id: 1, Name: "house"})
	builder.AddCatDbRow(
		fin.ExpenseCat,
		&categories.CatDbRow{Id: 2, Name: "car"})
	builder.AddCatDbRow(
		fin.ExpenseCat,
		&categories.CatDbRow{Id: 3, ParentId: 2, Name: "gas"})
	builder.AddCatDbRow(
		fin.ExpenseCat,
		&categories.CatDbRow{Id: 4, ParentId: 2, Name: "maintenance"})
	return builder.Build()
}
