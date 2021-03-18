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
	var entry fin.Entry
	var entryBalance fin.EntryBalance
	entry = fin.Entry{CatPayment: makeTotal(-400)}
	assert.True(withBalance(&entry, &entryBalance))
	assert.Equal(int64(347), entryBalance.Balance)
	entry = fin.Entry{CatPayment: makeTotal(-700)}
	assert.True(withBalance(&entry, &entryBalance))
	assert.Equal(int64(747), entryBalance.Balance)
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
	return fin.NewCatPayment(fin.NewCat("0:7"), -total, false, 0)
}
