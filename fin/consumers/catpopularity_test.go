package consumers

import (
	"testing"

	"github.com/keep94/finances/fin"
	"github.com/stretchr/testify/assert"
)

func TestCatPopularity(t *testing.T) {
	assert := assert.New(t)
	consumer := NewCatPopularityBuilder(3)
	var entry fin.Entry

	// Consume entry with trivial CatPayments doesn't count
	assert.True(consumer.CanConsume())
	consumer.Consume(entry)

	entry.CatPayment = fin.NewCatPayment(fin.NewCat("0:3"), 150, false, 1)
	assert.True(consumer.CanConsume())
	consumer.Consume(entry)

	entry.CatPayment = fin.NewCatPayment(fin.NewCat("0:4"), 225, false, 1)
	assert.True(consumer.CanConsume())
	consumer.Consume(entry)

	// Consume entry with trivial CatPayments doesn't count
	entry.CatPayment = fin.NewCatPayment(fin.Expense, 175, false, 1)
	assert.True(consumer.CanConsume())
	consumer.Consume(entry)

	var builder fin.CatPaymentBuilder
	builder.SetPaymentId(1)
	builder.AddCatRec(fin.CatRec{Cat: fin.NewCat("0:3")})
	builder.AddCatRec(fin.CatRec{Cat: fin.Expense})
	entry.CatPayment = builder.Build()
	assert.True(consumer.CanConsume())
	consumer.Consume(entry)

	assert.False(consumer.CanConsume())
	consumer.Consume(entry)

	popularities := consumer.Build()

	assert.Equal(
		fin.CatPopularity{
			fin.Expense: 1, fin.NewCat("0:3"): 2, fin.NewCat("0:4"): 1},
		popularities)
}

func TestCatPopularity_Empty(t *testing.T) {
	assert := assert.New(t)
	consumer := NewCatPopularityBuilder(3)
	assert.True(consumer.CanConsume())
	popularities := consumer.Build()
	assert.Equal(fin.CatPopularity{}, popularities)
}
