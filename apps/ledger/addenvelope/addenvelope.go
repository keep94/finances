package addenvelope

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"

	"github.com/keep94/finances/apps/ledger/common"
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/categories"
	"github.com/keep94/finances/fin/findb"
	"github.com/keep94/toolbox/http_util"
)

const (
	kAddEnvelope = "addenvelope"
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
<form method="post">
<input type="hidden" name="xsrf" value="{{.Xsrf}}">
<table>
  <tr>
    <td align="right">Category: </td>
    <td>
      <select name="cat" size="1">
{{with .GetSelection .CatSelectModel "cat"}}
        <option value="{{.Value}}">{{.Name}}</option>
{{end}}
        <option value="">--Select one--</option>
{{range .AvailableCatDetails}}
        <option value="{{.Id}}">{{.FullName}}</option>
{{end}}
      </select>
    </td>
  </tr>
  <tr>
    <td align="right">Amount: </td>
    <td><input type="text" name="amount" value="{{.Get "amount"}}"></td>
  </tr>
</table>
<input type="submit" name="save" value="Save">
<input type="submit" name="cancel" value="Cancel">
</form>
</div>
</body>
</html>`
)

var (
	kTemplate *template.Template
)

type Store interface {
	findb.AllocationsByYearRunner
	findb.AddAllocationRunner
}

type Handler struct {
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
		err = doPostAction(w, r, store)

		// If we get no error, we succesfully redirected back to envelopes
		// page, just return.
		if err == nil {
			return
		}
	}
	cds, _ := cdc.Get(nil)
	h.doGet(w, r, store, cds, err)
}

func (h *Handler) doGet(
	w http.ResponseWriter,
	r *http.Request,
	store findb.AllocationsByYearRunner,
	cds categories.CatDetailStore,
	err error) {
	leftnav := h.LN.Generate(w, r, common.SelectEnvelopes())
	if leftnav == "" {
		return
	}
	year, _ := strconv.Atoi(r.Form.Get("year"))
	var allocations map[int64]int64
	if common.Is21stCentury(year) {
		allocations, _ = store.AllocationsByYear(nil, int64(year))
	}
	catDetails := cds.ActiveCatDetails(false)
	catDetails = filterCatDetails(catDetails, allocations, cds)
	http_util.WriteTemplate(
		w,
		kTemplate,
		&view{
			http_util.Values{Values: r.Form},
			err,
			common.NewXsrfToken(r, kAddEnvelope),
			common.CatDisplayer{CatDetailStore: cds},
			catDetails,
			leftnav,
			h.Global})
}

func doSave(r *http.Request, store findb.AddAllocationRunner) error {
	if !common.VerifyXsrfToken(r, kAddEnvelope) {
		return common.ErrXsrf
	}
	year, _ := strconv.Atoi(r.Form.Get("year"))
	if !common.Is21stCentury(year) {
		return common.ErrInvalidYear
	}
	cat, caterr := fin.CatFromString(r.Form.Get("cat"))
	if caterr != nil || cat.Type != fin.ExpenseCat {
		return errors.New("Please choose an expense category.")
	}
	amount, amounterr := fin.ParseUSD(r.Form.Get("amount"))
	if amounterr != nil {
		return errors.New("Invalid amount.")
	}
	if amount < 0 {
		return errors.New("Amount must be non negative.")
	}
	return store.AddAllocation(nil, int64(year), cat.Id, amount)
}

func doPostAction(
	w http.ResponseWriter,
	r *http.Request,
	store findb.AddAllocationRunner) error {
	if http_util.HasParam(r.Form, "save") {
		if err := doSave(r, store); err != nil {
			return err
		}
	}
	prev := http_util.NewUrl("/fin/envelopes",
		"year", r.Form.Get("year"),
		"sort", r.Form.Get("sort"))
	http_util.Redirect(w, r, prev.String())
	return nil
}

func isPrincipalExpenseCat(cat fin.Cat, cds categories.CatDetailStore) bool {
	if cat == fin.Expense {
		return false
	}
	return cds.ImmediateParent(cat) == fin.Expense
}

// Return principal expense categories that don't already have envelopes.
// Leaves unfiltered unchanged.
func filterCatDetails(
	unfiltered []categories.CatDetail,
	allocs map[int64]int64,
	cds categories.CatDetailStore) []categories.CatDetail {
	var result []categories.CatDetail
	for _, cd := range unfiltered {
		cat := cd.Id()
		if !isPrincipalExpenseCat(cat, cds) {
			continue
		}
		if _, ok := allocs[cat.Id]; ok {
			continue
		}
		result = append(result, cd)
	}
	return result
}

type view struct {
	http_util.Values
	Error error
	Xsrf  string
	common.CatDisplayer
	AvailableCatDetails []categories.CatDetail
	LeftNav             template.HTML
	Global              *common.Global
}

func init() {
	kTemplate = common.NewTemplate("addenvelope", kTemplateSpec)
}
