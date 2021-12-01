package filters_test

import (
	"testing"

	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/filters"
	"github.com/stretchr/testify/assert"
)

func TestEntryFilterer(t *testing.T) {
	assert := assert.New(t)
	var entry fin.Entry
	filtererTrue := filters.EntryFilterer(
		func(ptr *fin.Entry) bool {
			return true
		})
	filtererFalse := filters.EntryFilterer(
		func(ptr *fin.Entry) bool {
			return false
		})
	assert.True(filtererTrue.Filter(&entry))
	assert.False(filtererFalse.Filter(&entry))
}

func TestRecurringEntryFilterer(t *testing.T) {
	assert := assert.New(t)
	var entry fin.RecurringEntry
	filtererTrue := filters.RecurringEntryFilterer(
		func(ptr *fin.RecurringEntry) bool {
			return true
		})
	filtererFalse := filters.RecurringEntryFilterer(
		func(ptr *fin.RecurringEntry) bool {
			return false
		})
	assert.True(filtererTrue.Filter(&entry))
	assert.False(filtererFalse.Filter(&entry))
}
