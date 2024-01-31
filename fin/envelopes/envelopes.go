// Package envelopes manages envelopes for tracking spending.
package envelopes

import (
	"fmt"
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

// Progress represents how far through the year an envelope is spent as
// month and day. For example, if an envelope is half spent, progress would
// be July 1 (7-01). If an envelope is 3/4 spent, progress would be
// Oct 1 (10-1).
type Progress struct {
	Month int64
	Day   int64
}

// ProgressOf returns the progress given spend and allocation in pennies.
// If allocation is zero or negative, calling IsDefined on returned
// Progress yields false.  If spend is negative and allocation is positve,
// ProgressOf returns "1-01" for the beginning of the year. When calculating,
// all months are considered 30 days so that all months are equal. If the
// day for February would be greater than 28, ProgressOf just returns "2-28."
// The month of returned progress will exceed 12 if spend exceeds allocation.
func ProgressOf(spend, allocation int64) Progress {
	if allocation <= 0 {
		return Progress{}
	}
	if spend < 0 {
		return Progress{Month: 1, Day: 1}
	}
	totalDays := int64(float64(spend) / float64(allocation) * 360.0)
	month := (totalDays / 30) + 1
	day := (totalDays % 30) + 1
	if month == 2 && day > 28 {
		day = 28
	}
	return Progress{Month: month, Day: day}
}

// IsDefined returns true if progress is defined. Progress is undefined
// if allocation for the envelope is 0 or negative.
func (p Progress) IsDefined() bool {
	return p.Month > 0 && p.Day > 0
}

// String returns progress as a string e.g "8-05" for Aug 5. If IsDefined()
// returns false, then String returns the empty string.
func (p Progress) String() string {
	if !p.IsDefined() {
		return ""
	}
	return fmt.Sprintf("%02d-%02d", p.Month, p.Day)
}

// Less returns true if p represents less progress than rhs. That is p comes
// earlier in the year than rhs.
func (p Progress) Less(rhs Progress) bool {
	if p.Month < rhs.Month {
		return true
	}
	if p.Month > rhs.Month {
		return false
	}
	return p.Day < rhs.Day
}

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

// Progress returns the progress for this envelope.
func (e *Envelope) Progress() Progress {
	return ProgressOf(e.Spent, e.Allocated)
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

// TotalProgress returns the total progress on all envelopes.
func (e Envelopes) TotalProgress() Progress {
	return ProgressOf(e.TotalSpent(), e.TotalAllocated())
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
func (s *Summary) TotalProgress() Progress {
	return ProgressOf(s.TotalSpent, s.Envelopes.TotalAllocated())
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
		func(i, j int) bool { return es[j].Progress().Less(es[i].Progress()) },
	)
}
