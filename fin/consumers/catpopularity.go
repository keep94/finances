package consumers

import (
	"github.com/keep94/consume2"
	"github.com/keep94/finances/fin"
)

// CatPopularityBuilder builds a fin.CatPopularity instance.
// CatPopularityBuilder implements consume2.Consumer[fin.Entry].
type CatPopularityBuilder struct {
	consumer     consume2.Consumer[fin.Entry]
	popularities catPopularityMap
}

// NewCatPopularityBuilder returns a consumer that consumes Entry values to
// build a fin.CatPopularity instance. The returned consumer consumes at most
// maxEntriesToRead values with categories other than the top level expense
// category and skips values that have only the top level expense category.
func NewCatPopularityBuilder(maxEntriesToRead int) *CatPopularityBuilder {
	popularities := make(catPopularityMap)
	consumer := consume2.Slice[fin.Entry](popularities, 0, maxEntriesToRead)
	consumer = consume2.Filter(consumer, nonTrivialCategories)
	return &CatPopularityBuilder{
		consumer: consumer, popularities: popularities}
}

func (c *CatPopularityBuilder) CanConsume() bool {
	return c.consumer.CanConsume()
}

func (c *CatPopularityBuilder) Consume(entry fin.Entry) {
	c.consumer.Consume(entry)
}

// Build builds the fin.CatPopularity instance.
func (c *CatPopularityBuilder) Build() fin.CatPopularity {
	result := make(fin.CatPopularity, len(c.popularities))
	for k, v := range c.popularities {
		result[k] = v
	}
	return result
}

type catPopularityMap map[fin.Cat]int

func (c catPopularityMap) CanConsume() bool {
	return true
}

func (c catPopularityMap) Consume(entry fin.Entry) {
	for _, catrec := range entry.CatRecs() {
		c[catrec.Cat]++
	}
}

func nonTrivialCategories(entry fin.Entry) bool {
	if entry.CatRecCount() > 1 {
		return true
	}
	if entry.CatRecCount() == 0 {
		return false
	}
	return entry.CatRecByIndex(0).Cat != fin.Expense
}
