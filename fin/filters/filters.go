// Package filters contains useful search filters.
package filters

import (
	"github.com/keep94/consume2"
	"github.com/keep94/finances/fin"
	"github.com/keep94/toolbox/str_util"
	"strings"
)

// WithBalance returns a function that maps a fin.Entry to a
// fin.EntryBalance. balance is the current balance of the account.
// fin.Entry values must be passed to returned function from newest to oldest.
func WithBalance(balance int64) func(entry fin.Entry) fin.EntryBalance {
	return func(entry fin.Entry) (result fin.EntryBalance) {
		result.Entry = entry
		result.Balance = balance
		balance -= entry.Total()
		return
	}
}

// AmountFilter filters by amount. Returns true if amt should be included or
// false otherwise.
type AmountFilter func(amt int64) bool

// AdvanceSearchSpec specifies what entries to search for.
// searches ignore case and whitespace.
type AdvanceSearchSpec struct {
	Name string
	Desc string
	// If non-zero, include only entries for given account.
	AccountId int64
	// If present, include only entries with line items that match CF.
	CF fin.CatFilter
	// If present, include only entries whose total matches AF.
	AF AmountFilter
}

// CompileAdvanceSearchSpec compiles a search specification.
func CompileAdvanceSearchSpec(
	spec *AdvanceSearchSpec) func(ptr *fin.Entry) bool {
	var filters []func(ptr *fin.Entry) bool
	if spec.AccountId != 0 {
		filters = append(filters, byAccountFilterer(spec.AccountId))
	}
	if spec.CF != nil {
		filters = append(filters, byCatFilterer(spec.CF))
	}
	if spec.AF != nil {
		filters = append(filters, byAmountFilterer(spec.AF))
	}
	if spec.Name != "" {
		filters = append(filters, byNameFilterer(str_util.Normalize(spec.Name)))
	}
	if spec.Desc != "" {
		filters = append(filters, byDescFilterer(str_util.Normalize(spec.Desc)))
	}
	return consume2.ComposeFilters(filters...)
}

func byAccountFilterer(accountId int64) func(ptr *fin.Entry) bool {
	return func(ptr *fin.Entry) bool {
		return ptr.WithPayment(accountId)
	}
}

func byCatFilterer(f fin.CatFilter) func(ptr *fin.Entry) bool {
	return func(ptr *fin.Entry) bool {
		return ptr.WithCat(f)
	}
}

func byAmountFilterer(f AmountFilter) func(ptr *fin.Entry) bool {
	return func(ptr *fin.Entry) bool {
		return f(ptr.Total())
	}
}

func byNameFilterer(name string) func(ptr *fin.Entry) bool {
	return func(ptr *fin.Entry) bool {
		return strings.Index(str_util.Normalize(ptr.Name), name) != -1
	}
}

func byDescFilterer(desc string) func(ptr *fin.Entry) bool {
	return func(ptr *fin.Entry) bool {
		return strings.Index(str_util.Normalize(ptr.Desc), desc) != -1
	}
}
