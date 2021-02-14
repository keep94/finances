package consumers

import (
	"github.com/keep94/finances/fin"
	"testing"
)

func TestFromCatPaymentAggregator(t *testing.T) {
	entries := []fin.Entry{
		{CatPayment: makeTotal(400)},
		{CatPayment: makeTotal(700)},
	}
	aggregator := catPaymentTotaler{}
	consumer := FromCatPaymentAggregator(&aggregator)
	entry := entries[0]
	consumer.Consume(&entry)
	entry = entries[1]
	consumer.Consume(&entry)
	if aggregator.total != 1100 {
		t.Errorf("Expected 1100, got %v", aggregator.total)
	}
}

func TestFromEntryAggregator(t *testing.T) {
	entries := []fin.Entry{
		{CatPayment: makeTotal(400)},
		{CatPayment: makeTotal(700)},
	}
	aggregator := entryTotaler{}
	consumer := FromEntryAggregator(&aggregator)
	entry := entries[0]
	consumer.Consume(&entry)
	entry = entries[1]
	consumer.Consume(&entry)
	if aggregator.total != 1100 {
		t.Errorf("Expected 1100, got %v", aggregator.total)
	}
}

func makeTotal(total int64) fin.CatPayment {
	return fin.NewCatPayment(fin.NewCat("0:7"), -total, false, 0)
}

type entryTotaler struct {
	total int64
}

func (e *entryTotaler) Include(entry *fin.Entry) {
	e.total += entry.Total()
}

type catPaymentTotaler struct {
	total int64
}

func (c *catPaymentTotaler) Include(cp *fin.CatPayment) {
	c.total += cp.Total()
}
