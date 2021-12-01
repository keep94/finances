// Package filters contains useful search filters.
package filters

import (
	"github.com/keep94/consume"
	"github.com/keep94/finances/fin"
	"github.com/keep94/toolbox/str_util"
	"strings"
)

// WithBalance returns a consume.Mapper that maps a fin.Entry to a
// fin.EntryBalance. balance is the current balance of the account.
// fin.Entry values must be passed to returned Mapper from newest to oldest.
// Returned Mapper maps each fin.Entry value to a fin.EntryBalance value
// that includes the balance at that fin.Entry value.
func WithBalance(balance int64) consume.Mapper {
	return &entryBalanceMapper{balance: balance}
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
		filters = append(filters, byAccountMapper(spec.AccountId))
	}
	if spec.CF != nil {
		filters = append(filters, byCatMapper(spec.CF))
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

func byAccountMapper(accountId int64) consume.Mapper {
	return EntryMapper(func(src, dest *fin.Entry) bool {
		*dest = *src
		return dest.WithPayment(accountId)
	})
}

func byCatMapper(f fin.CatFilter) consume.Mapper {
	return EntryMapper(func(src, dest *fin.Entry) bool {
		*dest = *src
		return dest.WithCat(f)
	})
}

func byAmountFilterer(f AmountFilter) consume.Filterer {
	return EntryFilterer(func(ptr *fin.Entry) bool {
		return f(ptr.Total())
	})
}

func byNameFilterer(name string) consume.Filterer {
	return EntryFilterer(func(ptr *fin.Entry) bool {
		return strings.Index(str_util.Normalize(ptr.Name), name) != -1
	})
}

func byDescFilterer(desc string) consume.Filterer {
	return EntryFilterer(func(ptr *fin.Entry) bool {
		return strings.Index(str_util.Normalize(ptr.Desc), desc) != -1
	})
}

type entryBalanceMapper struct {
	balance int64
	temp    fin.EntryBalance
}

func (e *entryBalanceMapper) Map(ptr interface{}) interface{} {
	p := ptr.(*fin.Entry)
	e.temp.Entry = *p
	e.temp.Balance = e.balance
	e.balance -= p.Total()
	return &e.temp
}

func (e *entryBalanceMapper) Clone() consume.Mapper {
	result := *e
	return &result
}
