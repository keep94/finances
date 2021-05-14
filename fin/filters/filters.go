// Package filters contains useful search filters.
package filters

import (
	"github.com/keep94/consume"
	"github.com/keep94/finances/fin"
	"github.com/keep94/toolbox/str_util"
	"strings"
)

// WithBalance returns a function that maps a fin.Entry to a fin.EntryBalance.
// balance is the current balance of the account.
// fin.Entry values must be passed to returned function from newest to oldest.
// Returned function maps each fin.Entry value to a fin.EntryBalance value
// that includes the balance at that fin.Entry value.
func WithBalance(balance int64) func(*fin.Entry, *fin.EntryBalance) bool {
	return func(src *fin.Entry, dest *fin.EntryBalance) bool {
		dest.Entry = *src
		dest.Balance = balance
		balance -= src.Total()
		return true
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
func CompileAdvanceSearchSpec(spec *AdvanceSearchSpec) consume.MapFilterer {
	var filters []interface{}
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
	return consume.NewMapFilterer(filters...)
}

func byAccountFilterer(accountId int64) func(src, dest *fin.Entry) bool {
	return func(src, dest *fin.Entry) bool {
		*dest = *src
		return dest.WithPayment(accountId)
	}
}

func byCatFilterer(f fin.CatFilter) func(src, dest *fin.Entry) bool {
	return func(src, dest *fin.Entry) bool {
		*dest = *src
		return dest.WithCat(f)
	}
}

func byAmountFilterer(f AmountFilter) func(*fin.Entry) bool {
	return func(ptr *fin.Entry) bool {
		return f(ptr.Total())
	}
}

func byNameFilterer(name string) func(*fin.Entry) bool {
	return func(ptr *fin.Entry) bool {
		return strings.Index(str_util.Normalize(ptr.Name), name) != -1
	}
}

func byDescFilterer(desc string) func(*fin.Entry) bool {
	return func(ptr *fin.Entry) bool {
		return strings.Index(str_util.Normalize(ptr.Desc), desc) != -1
	}
}
