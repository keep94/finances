// Package envelopes manages envelopes for tracking spending.
package envelopes

import (
	"sort"

	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/categories"
	"github.com/keep94/finances/fin/consumers"
	"github.com/keep94/finances/fin/findb"
	"github.com/keep94/toolbox/date_util"
	"github.com/keep94/toolbox/db"
)

const (
	kNonNilTransactionRequired = "non nil transaction required"
)

// Envelope represents a single yearly envelope.
type Envelope struct {

	// The expense Id of the envelope which corresponds to the Id field of
	// fin.Cat instances representing expenses
	ExpenseId int64

	// The name of the envelope
	Name string

	// Yearly allocation to envelope in pennies
	Allocated int64

	// Amount spent for the year from envelope in pennies
	Spent int64
}

// Remaining returns the amount remaining in envelope for the year in pennies.
func (e *Envelope) Remaining() int64 {
	return e.Allocated - e.Spent
}

// Progress returns what part of the year spending is at. For example, 800
// means spending is at beginning of August; 1250 means spending is at mid
// December. If allocation for this envelope is 0, Progress return 0. Note
// that normally progress returns at least 100 as that means beginning of
// January. If allocation is positive, and spending is negative, Progress
// returns 100 for beginning of January.
func (e *Envelope) Progress() int64 {
	return progress(e.Spent, e.Allocated)
}

// Ordering represents an ordering for envelopes.
type Ordering func([]*Envelope)

var (
	ByAllocatedDesc Ordering = byAllocatedDesc
	BySpentDesc     Ordering = bySpentDesc
	ByRemainingAsc  Ordering = byRemainingAsc
	ByProgressDesc  Ordering = byProgressDesc
)

// Envelopes represents a collection of envelopes for a year
type Envelopes []*Envelope

// Sort sorts these envelopes in place according to ordering.
func (e Envelopes) Sort(ordering Ordering) {
	ordering(e)
}

// Len returns the number of envelopes
func (e Envelopes) Len() int {
	return len(e)
}

// TotalAllocated returns the yearly allocation to all envelopes in pennies.
func (e Envelopes) TotalAllocated() int64 {
	result := int64(0)
	for _, envelope := range e {
		result += envelope.Allocated
	}
	return result
}

// TotalSpent returns the yearly spend from all envelopes in pennies.
func (e Envelopes) TotalSpent() int64 {
	result := int64(0)
	for _, envelope := range e {
		result += envelope.Spent
	}
	return result
}

// TotalRemaining returns the total amount remaining in all envelopes.
func (e Envelopes) TotalRemaining() int64 {
	return e.TotalAllocated() - e.TotalSpent()
}

// TotalProgress returns the total progress on all envelopes in the same
// way that Envelope.Progress returns the progress on a single envelope.
func (e Envelopes) TotalProgress() int64 {
	return progress(e.TotalSpent(), e.TotalAllocated())
}

func (e *Envelopes) add(envelope Envelope) {
	*e = append(*e, &envelope)
}

// Summary is the envelopes summary for a given year
type Summary struct {

	// The envelopes for the year
	Envelopes Envelopes

	// Total spent for the year in pennies including money not from envelopes
	TotalSpent int64
}

// Sort sorts the envelopes in this summary according to ordering.
func (s *Summary) Sort(ordering Ordering) {
	ordered := append(Envelopes(nil), s.Envelopes...)
	ordered.Sort(ordering)
	s.Envelopes = ordered
}

// UncategorizedSpend returns the amount spent for the year in pennies that
// didn't come out of the envelopes.
func (s *Summary) UncategorizedSpend() int64 {
	return s.TotalSpent - s.Envelopes.TotalSpent()
}

// TotalProgress returns the grand total progress which is similar to
// calling TotalProgress() on the envelopes except that it uses total
// spent from this instance instead of total spent from the envelopes.
func (s *Summary) TotalProgress() int64 {
	return progress(s.TotalSpent, s.Envelopes.TotalAllocated())
}

type Store interface {
	findb.EntriesRunner
	findb.AllocationsByYearRunner
}

// SummaryByYear returns the envelope summary for a given year and sorts the
// envelopes by name. t is the database transaction and must be non-nil.
// store represents the datastore. cds contains the categories. year is the
// year for which we are retrieving envelopes.
func SummaryByYear(
	t db.Transaction,
	store Store,
	cds categories.CatDetailStore,
	year int64) (*Summary, error) {
	if t == nil {
		panic(kNonNilTransactionRequired)
	}
	catTotals := make(fin.CatTotals)
	consumer := consumers.FromCatPaymentAggregator(catTotals)
	start := date_util.YMD(int(year), 1, 1)
	end := date_util.YMD(int(year+1), 1, 1)
	elo := &findb.EntryListOptions{Start: &start, End: &end}
	if err := store.Entries(t, elo, consumer); err != nil {
		return nil, err
	}
	rolledCatTotals, _ := cds.RollUp(catTotals)
	allocations, err := store.AllocationsByYear(t, year)
	if err != nil {
		return nil, err
	}
	catDetails := cds.DetailsByIds(allocationsToCatSet(allocations))
	var result Envelopes
	for _, cd := range catDetails {
		var e Envelope
		cat := cd.Id()
		e.ExpenseId = cat.Id
		e.Name = cd.FullName()
		e.Allocated = allocations[e.ExpenseId]
		e.Spent = rolledCatTotals[cat]
		result.add(e)
	}
	totalSpent := rolledCatTotals[fin.Expense]
	return &Summary{Envelopes: result, TotalSpent: totalSpent}, nil
}

func allocationsToCatSet(allocations map[int64]int64) fin.CatSet {
	result := make(fin.CatSet)
	for k, _ := range allocations {
		cat := fin.Cat{Id: k, Type: fin.ExpenseCat}
		result[cat] = true
	}
	return result
}

func progress(spent, allocated int64) int64 {
	if allocated <= 0 {
		return 0
	}
	if spent < 0 {
		return 100
	}
	prog := float64(spent)/float64(allocated)*12.0 + 1.0
	return int64(prog*100.0 + 0.5)
}

func byAllocatedDesc(es []*Envelope) {
	sort.Slice(
		es,
		func(i, j int) bool { return es[i].Allocated > es[j].Allocated },
	)
}

func bySpentDesc(es []*Envelope) {
	sort.Slice(
		es,
		func(i, j int) bool { return es[i].Spent > es[j].Spent },
	)
}

func byRemainingAsc(es []*Envelope) {
	sort.Slice(
		es,
		func(i, j int) bool {
			return es[i].Remaining() < es[j].Remaining()
		},
	)
}

func byProgressDesc(es []*Envelope) {
	sort.Slice(
		es,
		func(i, j int) bool { return es[i].Progress() > es[j].Progress() },
	)
}
