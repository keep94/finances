package fin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCatPopularity(t *testing.T) {
	assert := assert.New(t)
	var popularities CatPopularity
	consumer := BuildCatPopularity(3, &popularities)
	var entry Entry

	// Consume entry with trivial CatPayments doesn't count
	assert.True(consumer.CanConsume())
	consumer.Consume(&entry)

	entry.CatPayment = NewCatPayment(NewCat("0:3"), 150, false, 1)
	assert.True(consumer.CanConsume())
	consumer.Consume(&entry)

	entry.CatPayment = NewCatPayment(NewCat("0:4"), 225, false, 1)
	assert.True(consumer.CanConsume())
	consumer.Consume(&entry)

	// Consume entry with trivial CatPayments doesn't count
	entry.CatPayment = NewCatPayment(Expense, 175, false, 1)
	assert.True(consumer.CanConsume())
	consumer.Consume(&entry)

	var builder CatPaymentBuilder
	builder.SetPaymentId(1)
	builder.AddCatRec(CatRec{Cat: NewCat("0:3")})
	builder.AddCatRec(CatRec{Cat: Expense})
	entry.CatPayment = builder.Build()
	assert.True(consumer.CanConsume())
	consumer.Consume(&entry)

	assert.False(consumer.CanConsume())
	assert.Panics(func() { consumer.Consume(&entry) })

	assert.Nil(popularities)
	consumer.Finalize()
	assert.NotNil(popularities)
	assert.False(consumer.CanConsume())
	assert.Panics(func() { consumer.Consume(&entry) })

	assert.Equal(
		CatPopularity{Expense: 1, NewCat("0:3"): 2, NewCat("0:4"): 1},
		popularities)

	// test idempotency of Finalize
	popularities = nil
	consumer.Finalize()
	assert.Nil(popularities)

	consumer = BuildCatPopularity(3, &popularities)
	assert.True(consumer.CanConsume())
	consumer.Finalize()
	assert.NotNil(popularities)
	assert.False(consumer.CanConsume())
	assert.Panics(func() { consumer.Consume(&entry) })

	assert.Equal(CatPopularity{}, popularities)
}
