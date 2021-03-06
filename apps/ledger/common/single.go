package common

import (
	"errors"
	"fmt"
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/categories"
	"github.com/keep94/toolbox/date_util"
	"github.com/keep94/toolbox/http_util"
	"html/template"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	kMaxSplits = 10
)

var (
	// Error for date being wrong when adding an entry.
	ErrDateMayBeWrong         = errors.New("Date may be wrong, proceed anyway?")
	ErrConcurrentModification = errors.New(
		"Someone else already updated this entry. Click cancel and try again.")
)

var (
	entrySplits []EntrySplitType
)

// View for single entry pages.
type SingleEntryView struct {
	http_util.Values
	CatDisplayer
	Splits        []EntrySplitType
	Error         error
	Xsrf          string
	ExistingEntry bool
	Global        *Global
	LeftNav       template.HTML
	catPopularity fin.CatPopularity
}

func (v *SingleEntryView) ActiveCatDetails(
	showAccounts bool) []categories.CatDetail {
	return ActiveCatDetails(v.CatDetailStore, v.catPopularity, showAccounts)
}

// DateMayBeWrong returns true if and only if the error for this view is
// ErrDateMayBeWrong.
func (v *SingleEntryView) DateMayBeWrong() bool {
	return v.Error == ErrDateMayBeWrong
}

// EntrySplitType represents the display for a single split of an entry.
type EntrySplitType int

// CatParam returns the name of the category parameter for this split
func (s EntrySplitType) CatParam() string {
	return fmt.Sprintf("cat-%d", int(s))
}

// AmountParam returns the name of the amount parameter for this split
func (s EntrySplitType) AmountParam() string {
	return fmt.Sprintf("amount-%d", int(s))
}

// ReconcileParam returns the name of the reconciled parameter for this split
func (s EntrySplitType) ReconcileParam() string {
	return fmt.Sprintf("reconciled-%d", int(s))
}

// InitializeForm initializes an empty form for creating a brand new entry.
// If paymentId is non-zero, sets type payment combo box to the specified
// paymentId.
func InitializeForm(paymentId int64) url.Values {
	values := make(url.Values)
	if paymentId > 0 {
		values.Set("payment", strconv.FormatInt(paymentId, 10))
	}
	for _, split := range entrySplits {
		values.Set(split.CatParam(), fin.Expense.String())
	}
	return values
}

// ToSingleEntryView creates a view from a particular entry.
// The caller may safely add additional name value pairs to the Values field
// of returned view.
// The caller must refrain from making other types of changes to returned view.
func ToSingleEntryView(
	entry *fin.Entry,
	xsrf string,
	cds categories.CatDetailStore,
	catPopularity fin.CatPopularity,
	global *Global,
	leftnav template.HTML) *SingleEntryView {
	result := &SingleEntryView{
		Values:        http_util.Values{Values: make(url.Values)},
		CatDisplayer:  CatDisplayer{cds},
		Splits:        entrySplits,
		Error:         nil,
		Xsrf:          xsrf,
		ExistingEntry: true,
		Global:        global,
		LeftNav:       leftnav,
		catPopularity: catPopularity}
	result.Set("etag", strconv.FormatUint(entry.Etag, 10))
	result.Set("name", entry.Name)
	result.Set("desc", entry.Desc)
	result.Set("checkno", entry.CheckNo)
	result.Set("date", entry.Date.Format(date_util.YMDFormat))
	result.Set("payment", strconv.FormatInt(entry.PaymentId(), 10))
	if entry.Reconciled() {
		result.Set("reconciled", "on")
	}
	if entry.Status != fin.Reviewed {
		result.Set("need_review", "on")
	}
	catrecs := cds.SortedCatRecs(entry.CatRecs())
	for idx, split := range result.Splits {
		if idx < len(catrecs) {
			result.Set(split.CatParam(), catrecs[idx].Cat.String())
			result.Set(split.AmountParam(), fin.FormatUSD(catrecs[idx].Amount))
			if catrecs[idx].Reconciled {
				result.Set(split.ReconcileParam(), "on")
			}
		} else {
			result.Set(split.CatParam(), fin.Expense.String())
		}
	}
	return result
}

// ToSingleEntryViewFromForm creates a view from form data.
// existingEntry is true if the form data represents an existing entry or
// false if it represents a brand new entry.
// values are the form values.
// xsrf is the cross site request forgery token
// cds is the category detail store.
// catPopularity is the popularity of the categories. May be nil.
// global is the non changing global content of view.
// leftnav is the left navigation content
// err is the error from the form submission or nil if no error.
func ToSingleEntryViewFromForm(
	existingEntry bool,
	values url.Values,
	xsrf string,
	cds categories.CatDetailStore,
	catPopularity fin.CatPopularity,
	global *Global,
	leftnav template.HTML,
	err error) *SingleEntryView {
	return &SingleEntryView{
		Values:        http_util.Values{Values: values},
		CatDisplayer:  CatDisplayer{cds},
		Splits:        entrySplits,
		Error:         err,
		Xsrf:          xsrf,
		ExistingEntry: existingEntry,
		Global:        global,
		LeftNav:       leftnav,
		catPopularity: catPopularity}
}

// EntryMutation converts the form values from a single entry page into
// a mutation and returns that mutation or an error if the form values were
// invalid. Returned filter always returns true.
func EntryMutation(values url.Values) (
	mutation fin.EntryUpdater, err error) {
	date, err := time.Parse(date_util.YMDFormat, values.Get("date"))
	if err != nil {
		err = errors.New("Date in wrong format.")
		return
	}
	name := values.Get("name")
	if strings.TrimSpace(name) == "" {
		err = errors.New("Name required.")
		return
	}
	desc := values.Get("desc")
	checkno := values.Get("checkno")
	paymentId, _ := strconv.ParseInt(values.Get("payment"), 10, 64)
	if paymentId == 0 {
		err = errors.New("Missing payment.")
		return
	}
	cpb := fin.CatPaymentBuilder{}
	cpb.SetPaymentId(paymentId).SetReconciled(values.Get("reconciled") != "")
	catrec := fin.CatRec{}
	for _, split := range entrySplits {
		cat := fin.NewCat(values.Get(split.CatParam()))
		amountStr := values.Get(split.AmountParam())
		if amountStr == "" {
			break
		}
		var amount int64
		amount, err = fin.ParseUSD(amountStr)
		if err != nil {
			err = errors.New(fmt.Sprintf("Invalid amount: %s", amountStr))
			return
		}
		catrec = fin.CatRec{
			Cat:        cat,
			Amount:     amount,
			Reconciled: values.Get(split.ReconcileParam()) != ""}
		cpb.AddCatRec(catrec)
	}
	cp := cpb.Build()
	needReview := values.Get("need_review") != ""
	mutation = func(p *fin.Entry) bool {
		p.Date = date
		p.Name = name
		p.Desc = desc
		p.CheckNo = checkno
		p.CatPayment = cp
		if needReview {
			if p.Status == fin.Reviewed {
				p.Status = fin.NotReviewed
			}
		} else {
			if p.Status != fin.Reviewed {
				p.Status = fin.Reviewed
			}
		}
		return true
	}
	return
}

func init() {
	entrySplits = make([]EntrySplitType, kMaxSplits)
	for i := range entrySplits {
		entrySplits[i] = EntrySplitType(i)
	}
}
