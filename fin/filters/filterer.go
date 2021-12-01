package filters

import (
	"github.com/keep94/consume"
	"github.com/keep94/finances/fin"
)

type entryFilterer func(ptr *fin.Entry) bool

func EntryFilterer(f func(ptr *fin.Entry) bool) consume.Filterer {
	return entryFilterer(f)
}

func (e entryFilterer) Filter(ptr interface{}) bool {
	return e(ptr.(*fin.Entry))
}

type recurringEntryFilterer func(ptr *fin.RecurringEntry) bool

func RecurringEntryFilterer(
	f func(ptr *fin.RecurringEntry) bool) consume.Filterer {
	return recurringEntryFilterer(f)
}

func (r recurringEntryFilterer) Filter(ptr interface{}) bool {
	return r(ptr.(*fin.RecurringEntry))
}
