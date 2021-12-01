package filters

import (
	"testing"

	"github.com/keep94/consume"
	"github.com/keep94/finances/fin"
	"github.com/stretchr/testify/assert"
)

func TestWithBalance(t *testing.T) {
	assert := assert.New(t)
	withBalance := WithBalance(347)
	var left, right []fin.EntryBalance
	consumer := consume.Compose(
		consume.MapFilter(consume.AppendTo(&left), withBalance),
		consume.MapFilter(consume.AppendTo(&right), withBalance),
	)
	var entry fin.Entry
	entry = fin.Entry{CatPayment: makeTotal(-400)}
	consumer.Consume(&entry)
	entry = fin.Entry{CatPayment: makeTotal(-700)}
	consumer.Consume(&entry)
	assert.Equal(int64(347), left[0].Balance)
	assert.Equal(int64(347), right[0].Balance)
	assert.Equal(int64(747), left[1].Balance)
	assert.Equal(int64(747), right[1].Balance)
}

func TestCompileAdvanceSearchSpec(t *testing.T) {
	if output := runFilter(CompileAdvanceSearchSpec(
		&AdvanceSearchSpec{
			Name: "Name"})); output != 3 {
		t.Errorf("Expected 3, got %v", output)
	}
	if output := runFilter(CompileAdvanceSearchSpec(
		&AdvanceSearchSpec{
			Name: "Name",
			Desc: "Desc"})); output != 2 {
		t.Errorf("Expected 2, got %v", output)
	}
	if output := runFilter(CompileAdvanceSearchSpec(
		&AdvanceSearchSpec{
			Name: "Name",
			Desc: "Desc",
			CF:   func(c fin.Cat) bool { return c == fin.NewCat("0:7") }})); output != 1 {
		t.Errorf("Expected 1, got %v", output)
	}
	if output := runFilter(CompileAdvanceSearchSpec(
		&AdvanceSearchSpec{
			AF: func(amt int64) bool { return amt == -200 }})); output != 1 {
		t.Errorf("Expected 1, got %v", output)
	}
	if output := runFilter(CompileAdvanceSearchSpec(
		&AdvanceSearchSpec{
			AF: func(amt int64) bool { return amt == -201 }})); output != 0 {
		t.Errorf("Expected 0, got %v", output)
	}
}

func TestAccountFiltering(t *testing.T) {
	assert := assert.New(t)
	var builder fin.CatPaymentBuilder
	builder.AddCatRec(fin.CatRec{Cat: fin.NewCat("0:302"), Amount: 4700})
	builder.AddCatRec(fin.CatRec{Cat: fin.NewCat("2:3"), Amount: 3400})
	builder.SetPaymentId(5)
	entry := fin.Entry{CatPayment: builder.Build()}

	filterer := CompileAdvanceSearchSpec(&AdvanceSearchSpec{AccountId: 0})
	result := filterer.MapFilter(&entry).(*fin.Entry)
	assert.Equal(int64(-8100), result.Total())

	filterer = CompileAdvanceSearchSpec(&AdvanceSearchSpec{AccountId: 5})
	result = filterer.MapFilter(&entry).(*fin.Entry)
	assert.Equal(int64(-8100), result.Total())

	filterer = CompileAdvanceSearchSpec(&AdvanceSearchSpec{AccountId: 3})
	result = filterer.MapFilter(&entry).(*fin.Entry)
	assert.Equal(int64(3400), result.Total())

	filterer = CompileAdvanceSearchSpec(&AdvanceSearchSpec{AccountId: 2})
	assert.Nil(filterer.MapFilter(&entry))

	filterer = CompileAdvanceSearchSpec(&AdvanceSearchSpec{AccountId: -4})
	assert.Nil(filterer.MapFilter(&entry))

	catIs302 := func(c fin.Cat) bool { return c == fin.NewCat("0:302") }
	amountIsNeg4700 := func(amt int64) bool { return amt == -4700 }
	amountIsNeg8100 := func(amt int64) bool { return amt == -8100 }

	filterer = CompileAdvanceSearchSpec(
		&AdvanceSearchSpec{AccountId: 5, CF: catIs302, AF: amountIsNeg4700})
	result = filterer.MapFilter(&entry).(*fin.Entry)
	assert.Equal(int64(-4700), result.Total())

	filterer = CompileAdvanceSearchSpec(
		&AdvanceSearchSpec{AccountId: 5, CF: catIs302, AF: amountIsNeg8100})
	assert.Nil(filterer.MapFilter(&entry))

	filterer = CompileAdvanceSearchSpec(
		&AdvanceSearchSpec{AccountId: 3, CF: catIs302})
	assert.Nil(filterer.MapFilter(&entry))
}

func runFilter(f consume.MapFilterer) int {
	result := 0
	if f.MapFilter(&fin.Entry{Name: "Name 1", Desc: "Desc 1"}) != nil {
		result++
	}
	if f.MapFilter(&fin.Entry{Name: "Name 2", Desc: "Other"}) != nil {
		result++
	}
	if f.MapFilter(&fin.Entry{Name: "Other", Desc: "Other"}) != nil {
		result++
	}
	if f.MapFilter(&fin.Entry{
		Name:       "Name 3",
		Desc:       "Desc 3",
		CatPayment: makeTotal(-200)}) != nil {
		result++
	}
	return result
}

func makeTotal(total int64) fin.CatPayment {
	return fin.NewCatPayment(fin.NewCat("0:7"), -total, false, 17)
}
