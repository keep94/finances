package envelopes

import (
	"html/template"
	"net/http"
	"net/url"
	"strconv"

	"github.com/keep94/finances/apps/ledger/common"
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/categories"
	edata "github.com/keep94/finances/fin/envelopes"
	"github.com/keep94/finances/fin/findb"
	"github.com/keep94/toolbox/db"
	"github.com/keep94/toolbox/http_util"
)

const (
	kEnvelopes = "envelopes"
)

var (
	kTemplateSpec = `
<html>
<head>
  <title>{{.Global.Title}}</title>
  {{if .Global.Icon}}
    <link rel="shortcut icon" href="/images/favicon.ico" type="image/x-icon" />
  {{end}}
  <link rel="stylesheet" type="text/css" href="/static/theme.css" />
</head>
<body>
{{.LeftNav}}
<div class="main">
{{with .Error}}
  <span class="error">{{.Error}}</span>
{{end}}
<form>
  <table>
    <tr>
      <td>Year: </td>
      <td><input type="text" name="year" value="{{.Get "year"}}"></td>
      <td><input type="submit" value="Get Envelopes"></td>
    </tr>
  </table>
</form>
<hr>
{{with .AddEnvelopeUrl}}
<a href="{{.}}">New Envelope</a><br><br>
{{end}}
{{if .Summary}}
Envelope Count: {{.Summary.Envelopes.Len}}<br>
Total Allocated: {{FormatUSDRaw .Summary.Envelopes.TotalAllocated}}<br>
Total Spent: {{FormatUSDRaw .Summary.Envelopes.TotalSpent}}<br>
Total Remaining: {{FormatUSD .Summary.Envelopes.TotalRemaining}}<br>
{{with .Summary.Envelopes.TotalProgress}}
Total Progress: {{FormatUSDRaw .}}<br>
{{else}}
Total Progress: --<br>
{{end}}
<br>
Uncategorized Spend: {{FormatUSDRaw .Summary.UncategorizedSpend}}<br>
Grand Total Spent: {{FormatUSDRaw .Summary.TotalSpent}}<br>
{{with .Summary.TotalProgress}}
Grand Total Progress: {{FormatUSDRaw .}}<br>
{{else}}
Grand Total Progress: --<br>
{{end}}
<br>
<table border=1>
  <tr>
    <th><a href="{{.SortByName}}">Name</a></th>
    <th><a href="{{.SortByAlloc}}">Allocated</a></th>
    <th><a href="{{.SortBySpent}}">Spent</a></th>
    <th><a href="{{.SortByRemaining}}">Remaining</a></th>
    <th><a href="{{.SortByProgress}}">Progress</a></th>
    <th>Delete</th>
  </tr>
{{with $top := .}}
  {{range .Summary.Envelopes}}
    <tr>
      <td><a href="{{$top.TransactionsLink .ExpenseId}}">{{.Name}}</a></td>
      <td align="right">{{FormatUSDRaw .Allocated}}</td>
      <td align="right">{{FormatUSDRaw .Spent}}</td>
      <td align="right">{{FormatUSD .Remaining}}</td>
{{with .Progress}}
      <td align="right">{{FormatUSDRaw .}}</td>
{{else}}
      <td align="right">--</td>
{{end}}
      <td>
        <form method="POST">
          <input type="hidden" name="xsrf" value="{{$top.Xsrf}}">
          <input type="hidden" name="expenseId" value="{{.ExpenseId}}">
          <input type="submit" value="X" onclick="return confirm('Are you sure you want to delete this envelope?');">
        </form>
      </td>
    </tr>
  {{end}}
{{end}}
</table>
{{end}}
</div>
</body>
</html>`
)

const (
	kSort      = "sort"
	kName      = "name"
	kAllocated = "allocated"
	kSpent     = "spent"
	kRemaining = "remaining"
	kProgress  = "progress"
)

var (
	kTemplate *template.Template

	kOrderings = map[string]edata.Ordering{
		kAllocated: edata.ByAllocatedDesc,
		kSpent:     edata.BySpentDesc,
		kRemaining: edata.ByRemainingAsc,
		kProgress:  edata.ByProgressDesc,
	}
)

type Store interface {
	edata.Store
	findb.RemoveAllocationRunner
}

type Handler struct {
	Doer   db.Doer
	LN     *common.LeftNav
	Global *common.Global
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	session := common.GetUserSession(r)
	cdc := session.Cache
	store := session.Store.(Store)
	var err error
	if r.Method == "POST" {
		err = doPostAction(r, store)
	}
	cds, _ := cdc.Get(nil)
	h.doGet(w, r, store, cds, err)
}

func (h *Handler) doGet(
	w http.ResponseWriter,
	r *http.Request,
	store edata.Store,
	cds categories.CatDetailStore,
	err error) {
	leftnav := h.LN.Generate(w, r, common.SelectEnvelopes())
	if leftnav == "" {
		return
	}
	year, _ := strconv.Atoi(r.Form.Get("year"))
	summary, gerr := h.getSummary(store, cds, year, r.Form.Get("sort"))
	if gerr != nil {
		err = gerr
	}
	var addEnvelopeUrl *url.URL
	if common.Is21stCentury(year) {
		addEnvelopeUrl = http_util.NewUrl(
			"/fin/addenvelope",
			"year", strconv.Itoa(year),
			kSort, r.Form.Get(kSort))
	}
	http_util.WriteTemplate(
		w,
		kTemplate,
		&view{
			r.Form,
			err,
			common.NewXsrfToken(r, kEnvelopes),
			addEnvelopeUrl,
			summary,
			http_util.WithParams(r.URL, kSort, kName),
			http_util.WithParams(r.URL, kSort, kAllocated),
			http_util.WithParams(r.URL, kSort, kSpent),
			http_util.WithParams(r.URL, kSort, kRemaining),
			http_util.WithParams(r.URL, kSort, kProgress),
			envelopeLinker(year),
			leftnav,
			h.Global})
}

func (h *Handler) getSummary(
	store edata.Store,
	cds categories.CatDetailStore,
	year int,
	order string) (result *edata.Summary, err error) {
	if !common.Is21stCentury(year) {
		return nil, common.ErrInvalidYear
	}
	err = h.Doer.Do(func(t db.Transaction) (err error) {
		result, err = edata.SummaryByYear(t, store, cds, int64(year))
		return
	})
	if err != nil {
		return nil, err
	}
	result = withSortedEnvelopes(result, order)
	return
}

func withSortedEnvelopes(
	summary *edata.Summary, order string) *edata.Summary {
	result := *summary
	if ordering, ok := kOrderings[order]; ok {
		result.Sort(ordering)
	}
	return &result
}

func doPostAction(r *http.Request, store findb.RemoveAllocationRunner) error {
	if !common.VerifyXsrfToken(r, kEnvelopes) {
		return common.ErrXsrf
	}
	year, _ := strconv.Atoi(r.Form.Get("year"))
	expenseId, _ := strconv.ParseInt(r.Form.Get("expenseId"), 10, 64)
	return store.RemoveAllocation(nil, int64(year), expenseId)
}

type view struct {
	url.Values
	Error           error
	Xsrf            string
	AddEnvelopeUrl  *url.URL
	Summary         *edata.Summary
	SortByName      *url.URL
	SortByAlloc     *url.URL
	SortBySpent     *url.URL
	SortByRemaining *url.URL
	SortByProgress  *url.URL
	envelopeLinker
	LeftNav template.HTML
	Global  *common.Global
}

type envelopeLinker int

func (e envelopeLinker) TransactionsLink(expenseId int64) *url.URL {
	cat := fin.Cat{Id: expenseId, Type: fin.ExpenseCat}
	return http_util.NewUrl("/fin/list",
		"sd", strconv.Itoa(int(e)),
		"ed", strconv.Itoa(int(e+1)),
		"cat", cat.String())
}

func init() {
	kTemplate = common.NewTemplate("envelopes", kTemplateSpec)
}
