package consumers

import (
	"github.com/keep94/consume"
	"github.com/keep94/finances/fin"
)

// BuildCatPopularity returns a consumer that consumes Entry values to
// build a CatPopularity instance. The returned consumer consumes at most
// maxEntriesToRead values with categories other than the top level expense
// category and skips values that have only the top level expense category.
// Caller must call Finalize on returned consumer for the built CatPopularity
// instance to be stored at catPopularity.
func BuildCatPopularity(
	maxEntriesToRead int,
	catPopularity *fin.CatPopularity) consume.ConsumeFinalizer {
	popularities := make(catPopularityMap)
	consumer := consume.Slice(popularities, 0, maxEntriesToRead)
	consumer = consume.MapFilter(consumer, nonTrivialCategories)
	return &catPopularityConsumer{
		Consumer: consumer, popularities: popularities, result: catPopularity}
}

type catPopularityMap map[fin.Cat]int

func (c catPopularityMap) CanConsume() bool {
	return true
}

func (c catPopularityMap) Consume(ptr interface{}) {
	entry := ptr.(*fin.Entry)
	for _, catrec := range entry.CatRecs() {
		c[catrec.Cat]++
	}
}

func nonTrivialCategories(entry *fin.Entry) bool {
	if entry.CatRecCount() > 1 {
		return true
	}
	if entry.CatRecCount() == 0 {
		return false
	}
	return entry.CatRecByIndex(0).Cat != fin.Expense
}

type catPopularityConsumer struct {
	consume.Consumer
	popularities catPopularityMap
	result       *fin.CatPopularity
	finalized    bool
}

func (c *catPopularityConsumer) Finalize() {
	if c.finalized {
		return
	}
	c.finalized = true
	c.Consumer = consume.Nil()
	*c.result = fin.CatPopularity(c.popularities)
}
