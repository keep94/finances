package filters

import (
	"github.com/keep94/consume"
	"github.com/keep94/finances/fin"
)

type entryMapper struct {
	M    func(src, dest *fin.Entry) bool
	temp fin.Entry
}

func EntryMapper(m func(src, dest *fin.Entry) bool) consume.Mapper {
	return &entryMapper{M: m}
}

func (e *entryMapper) Map(ptr interface{}) interface{} {
	if e.M(ptr.(*fin.Entry), &e.temp) {
		return &e.temp
	}
	return nil
}

func (e *entryMapper) Clone() consume.Mapper {
	result := *e
	return &result
}

type entryBalanceIntMapper struct {
	M    func(src *fin.EntryBalance, dest *int) bool
	temp int
}

func EntryBalanceIntMapper(
	m func(src *fin.EntryBalance, dest *int) bool) consume.Mapper {
	return &entryBalanceIntMapper{M: m}
}

func (e *entryBalanceIntMapper) Map(ptr interface{}) interface{} {
	if e.M(ptr.(*fin.EntryBalance), &e.temp) {
		return &e.temp
	}
	return nil
}

func (e *entryBalanceIntMapper) Clone() consume.Mapper {
	result := *e
	return &result
}

type recurringEntryMapper struct {
	M    func(src, dest *fin.RecurringEntry) bool
	temp fin.RecurringEntry
}

func RecurringEntryMapper(
	m func(src, dest *fin.RecurringEntry) bool) consume.Mapper {
	return &recurringEntryMapper{M: m}
}

func (r *recurringEntryMapper) Map(ptr interface{}) interface{} {
	if r.M(ptr.(*fin.RecurringEntry), &r.temp) {
		return &r.temp
	}
	return nil
}

func (r *recurringEntryMapper) Clone() consume.Mapper {
	result := *r
	return &result
}
