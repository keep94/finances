// Package consumers contains useful consumers
package consumers

import (
	"github.com/keep94/consume2"
	"github.com/keep94/finances/fin"
)

// CatPaymentAggregator aggregates CatPayment values.
type CatPaymentAggregator interface {
	Include(cp fin.CatPayment)
}

// EntryAggregator aggregates Entry values.
type EntryAggregator interface {
	Include(entry fin.Entry)
}

// FromCatPaymentAggregator converts a CatPaymentAggregator to a Consumer of
// fin.Entry values.
func FromCatPaymentAggregator(
	aggregator CatPaymentAggregator) consume2.Consumer[fin.Entry] {
	return consume2.ConsumerFunc[fin.Entry](func(entry fin.Entry) {
		aggregator.Include(entry.CatPayment)
	})
}

// FromEntryAggregator converts a EntryAggregator to a Consumer of
// fin.Entry values.
func FromEntryAggregator(
	aggregator EntryAggregator) consume2.Consumer[fin.Entry] {
	return consume2.ConsumerFunc[fin.Entry](aggregator.Include)
}
