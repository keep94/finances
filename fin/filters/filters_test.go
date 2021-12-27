package filters

import (
	"testing"

	"github.com/keep94/consume2"
	"github.com/keep94/finances/fin"
	"github.com/stretchr/testify/assert"
)

func TestWithBalance(t *testing.T) {
	assert := assert.New(t)
	var ebs []fin.EntryBalance
	consumer := consume2.Map(consume2.AppendTo(&ebs), WithBalance(347))
	consumer.Consume(fin.Entry{CatPayment: makeTotal(-400)})
	consumer.Consume(fin.Entry{CatPayment: makeTotal(-700)})
	assert.Equal(int64(347), ebs[0].Balance)
	assert.Equal(int64(747), ebs[1].Balance)
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
	result := entry
	assert.True(filterer(&result))
	assert.Equal(int64(-8100), result.Total())

	filterer = CompileAdvanceSearchSpec(&AdvanceSearchSpec{AccountId: 5})
	result = entry
	assert.True(filterer(&result))
	assert.Equal(int64(-8100), result.Total())

	filterer = CompileAdvanceSearchSpec(&AdvanceSearchSpec{AccountId: 3})
	result = entry
	assert.True(filterer(&result))
	assert.Equal(int64(3400), result.Total())

	filterer = CompileAdvanceSearchSpec(&AdvanceSearchSpec{AccountId: 2})
	result = entry
	assert.False(filterer(&result))

	filterer = CompileAdvanceSearchSpec(&AdvanceSearchSpec{AccountId: -4})
	result = entry
	assert.False(filterer(&result))

	catIs302 := func(c fin.Cat) bool { return c == fin.NewCat("0:302") }
	amountIsNeg4700 := func(amt int64) bool { return amt == -4700 }
	amountIsNeg8100 := func(amt int64) bool { return amt == -8100 }

	filterer = CompileAdvanceSearchSpec(
		&AdvanceSearchSpec{AccountId: 5, CF: catIs302, AF: amountIsNeg4700})
	result = entry
	assert.True(filterer(&result))
	assert.Equal(int64(-4700), result.Total())

	filterer = CompileAdvanceSearchSpec(
		&AdvanceSearchSpec{AccountId: 5, CF: catIs302, AF: amountIsNeg8100})
	result = entry
	assert.False(filterer(&result))

	filterer = CompileAdvanceSearchSpec(
		&AdvanceSearchSpec{AccountId: 3, CF: catIs302})
	result = entry
	assert.False(filterer(&result))
}

func runFilter(f func(ptr *fin.Entry) bool) int {
	result := 0
	if f(&fin.Entry{Name: "Name 1", Desc: "Desc 1"}) {
		result++
	}
	if f(&fin.Entry{Name: "Name 2", Desc: "Other"}) {
		result++
	}
	if f(&fin.Entry{Name: "Other", Desc: "Other"}) {
		result++
	}
	if f(&fin.Entry{
		Name:       "Name 3",
		Desc:       "Desc 3",
		CatPayment: makeTotal(-200)}) {
		result++
	}
	return result
}

func makeTotal(total int64) fin.CatPayment {
	return fin.NewCatPayment(fin.NewCat("0:7"), -total, false, 17)
}
