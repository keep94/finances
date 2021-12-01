package filters_test

import (
	"testing"

	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/filters"
	"github.com/stretchr/testify/assert"
)

func TestEntryMapper(t *testing.T) {
	assert := assert.New(t)
	var entry fin.Entry
	mapperTrue := filters.EntryMapper(
		func(src, dest *fin.Entry) bool {
			return true
		})
	mapperFalse := filters.EntryMapper(
		func(src, dest *fin.Entry) bool {
			return false
		})
	assert.Nil(mapperFalse.Map(&entry))
	assert.NotNil(mapperTrue.Map(&entry))
	assert.Same(mapperTrue.Map(&entry), mapperTrue.Map(&entry))
	assert.NotSame(mapperTrue.Map(&entry), mapperTrue.Clone().Map(&entry))
}

func TestEntryBalanceIntMapper(t *testing.T) {
	assert := assert.New(t)
	var entry fin.EntryBalance
	mapperTrue := filters.EntryBalanceIntMapper(
		func(src *fin.EntryBalance, dest *int) bool {
			return true
		})
	mapperFalse := filters.EntryBalanceIntMapper(
		func(src *fin.EntryBalance, dest *int) bool {
			return false
		})
	assert.Nil(mapperFalse.Map(&entry))
	assert.NotNil(mapperTrue.Map(&entry))
	assert.Same(mapperTrue.Map(&entry), mapperTrue.Map(&entry))
	assert.NotSame(mapperTrue.Map(&entry), mapperTrue.Clone().Map(&entry))
}

func TestRecurringEntryMapper(t *testing.T) {
	assert := assert.New(t)
	var entry fin.RecurringEntry
	mapperTrue := filters.RecurringEntryMapper(
		func(src, dest *fin.RecurringEntry) bool {
			return true
		})
	mapperFalse := filters.RecurringEntryMapper(
		func(src, dest *fin.RecurringEntry) bool {
			return false
		})
	assert.Nil(mapperFalse.Map(&entry))
	assert.NotNil(mapperTrue.Map(&entry))
	assert.Same(mapperTrue.Map(&entry), mapperTrue.Map(&entry))
	assert.NotSame(mapperTrue.Map(&entry), mapperTrue.Clone().Map(&entry))
}
