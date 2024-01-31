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

func TestProgress(t *testing.T) {
	p := ProgressOf(329, 3600)
	assert.Equal(t, Progress{Month: 2, Day: 3}, p)
}

func TestProgressWayOver(t *testing.T) {
	p := ProgressOf(1007, 120)
	assert.Equal(t, "101-22", p.String())
}

func TestProgressOverflow(t *testing.T) {
	bigInt := int64(1) << 62
	p := ProgressOf(bigInt/3+1, bigInt)
	assert.Equal(t, Progress{Month: 5, Day: 1}, p)
}

func TestProgressFebruary(t *testing.T) {
	p := ProgressOf(599, 3600)
	assert.Equal(t, Progress{Month: 2, Day: 28}, p)
}

func TestProgressMarch(t *testing.T) {
	p := ProgressOf(299, 1200)
	assert.Equal(t, Progress{Month: 3, Day: 30}, p)
	assert.True(t, p.IsDefined())
}

func TestProgressNegative(t *testing.T) {
	p := ProgressOf(-79, 1200)
	assert.Equal(t, Progress{Month: 1, Day: 1}, p)
	assert.True(t, p.IsDefined())
}

func TestProgressUndefined(t *testing.T) {
	p := ProgressOf(343, 0)
	assert.Equal(t, Progress{}, p)
	assert.False(t, p.IsDefined())
	assert.Empty(t, p.String())
}

func TestProgressString(t *testing.T) {
	p := Progress{Month: 11, Day: 4}
	assert.Equal(t, "11-04", p.String())
	p = Progress{Month: 5, Day: 31}
	assert.Equal(t, "05-31", p.String())
}

func TestProgressLess(t *testing.T) {
	p := Progress{Month: 4, Day: 15}
	assert.True(t, p.Less(Progress{Month: 5, Day: 1}))
	assert.True(t, p.Less(Progress{Month: 4, Day: 16}))
	assert.False(t, p.Less(Progress{Month: 4, Day: 15}))
	assert.False(t, p.Less(Progress{Month: 3, Day: 30}))
}

func TestEnvelope(t *testing.T) {
	e := Envelope{Allocated: 6000, Spent: 2000}
	assert.Equal(t, int64(4000), e.Remaining())
	assert.Equal(t, Progress{Month: 5, Day: 1}, e.Progress())
}

func TestEnvelopeNoAlloc(t *testing.T) {
	e := Envelope{Spent: 2000}
	assert.Equal(t, int64(-2000), e.Remaining())
	assert.False(t, e.Progress().IsDefined())
}

func TestEnvelopes(t *testing.T) {
	var es Envelopes
	es.add(Envelope{Allocated: 6000, Spent: 4000})
	es.add(Envelope{Allocated: 2000, Spent: 1000})
	assert.Equal(t, 2, es.Len())
	assert.Equal(t, int64(8000), es.TotalAllocated())
	assert.Equal(t, int64(5000), es.TotalSpent())
	assert.Equal(t, int64(3000), es.TotalRemaining())
	assert.Equal(t, Progress{Month: 8, Day: 16}, es.TotalProgress())
}

func TestEnvelopesEmpty(t *testing.T) {
	var es Envelopes
	assert.Equal(t, int64(0), es.TotalAllocated())
	assert.Equal(t, int64(0), es.TotalSpent())
	assert.Equal(t, int64(0), es.TotalRemaining())
	assert.False(t, es.TotalProgress().IsDefined())
}

func TestEnvelopesSort(t *testing.T) {
	e1 := &Envelope{Allocated: 10000, Spent: 19999}
	e2 := &Envelope{Allocated: 30000, Spent: 10000}
	e3 := &Envelope{Allocated: 20000, Spent: 30000}
	e4 := &Envelope{Allocated: 40000, Spent: 40000}
	es := Envelopes{e1, e2, e3, e4}
	es.Sort(ByAllocatedDesc)
	assert.Equal(t, Envelopes{e4, e2, e3, e1}, es)
	es.Sort(BySpentDesc)
	assert.Equal(t, Envelopes{e4, e3, e1, e2}, es)
	es.Sort(ByRemainingAsc)
	assert.Equal(t, Envelopes{e3, e1, e4, e2}, es)
	es.Sort(ByProgressDesc)
	assert.Equal(t, Envelopes{e1, e3, e4, e2}, es)
}

func TestSummary(t *testing.T) {
	var es Envelopes
	es.add(Envelope{Allocated: 3000, Spent: 5000})
	es.add(Envelope{Allocated: 9000, Spent: 2000})
	summary := &Summary{Envelopes: es, TotalSpent: 8000}
	assert.Equal(t, int64(1000), summary.UncategorizedSpend())
	assert.Equal(t, Progress{Month: 9, Day: 1}, summary.TotalProgress())
}

func TestSummarySort(t *testing.T) {
	e1 := &Envelope{Allocated: 10000, Spent: 19999}
	e2 := &Envelope{Allocated: 30000, Spent: 10000}
	e3 := &Envelope{Allocated: 20000, Spent: 30000}
	e4 := &Envelope{Allocated: 40000, Spent: 40000}
	es := Envelopes{e1, e2, e3, e4}
	summary := Summary{Envelopes: es, TotalSpent: 100000}
	sortedSummary := summary
	sortedSummary.Sort(ByAllocatedDesc)
	assert.NotEqual(t, summary, sortedSummary)
	assert.Equal(t, Envelopes{e4, e2, e3, e1}, sortedSummary.Envelopes)
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
