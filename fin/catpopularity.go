package fin

import (
	"github.com/keep94/consume"
)

// CatPopularity values are immutable by contract. The key is the category;
// the value is greater than or equal to zero and indicates popularity of
// the category.
type CatPopularity map[Cat]int

// BuildCatPopularity returns a consumer that consumes Entry values to
// build a CatPopularity instance. The returned consumer consumes at most
// maxEntriesToRead values with categories other than the top level expense
// category and skips values that have only the top level expense category.
// Caller must call Finalize on returned consumer for the built CatPopularity
// instance to be stored at catPopularity.
func BuildCatPopularity(
	maxEntriesToRead int,
	catPopularity *CatPopularity) consume.ConsumeFinalizer {
	popularities := make(catPopularityMap)
	consumer := consume.Slice(popularities, 0, maxEntriesToRead)
	consumer = consume.MapFilter(consumer, nonTrivialCategories)
	return &catPopularityConsumer{
		Consumer: consumer, popularities: popularities, result: catPopularity}
}

type catPopularityMap map[Cat]int

func (c catPopularityMap) CanConsume() bool {
	return true
}

func (c catPopularityMap) Consume(ptr interface{}) {
	entry := ptr.(*Entry)
	for _, catrec := range entry.CatRecs() {
		c[catrec.Cat]++
	}
}

func nonTrivialCategories(entry *Entry) bool {
	if entry.CatRecCount() > 1 {
		return true
	}
	if entry.CatRecCount() == 0 {
		return false
	}
	return entry.CatRecByIndex(0).Cat != Expense
}

type catPopularityConsumer struct {
	consume.Consumer
	popularities catPopularityMap
	result       *CatPopularity
	finalized    bool
}

func (c *catPopularityConsumer) Finalize() {
	if c.finalized {
		return
	}
	c.finalized = true
	c.Consumer = consume.Nil()
	*c.result = CatPopularity(c.popularities)
}
