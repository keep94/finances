package unreconciled

import (
	"fmt"
	"github.com/keep94/consume2"
	"github.com/keep94/finances/apps/ledger/common"
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/categories"
	"github.com/keep94/finances/fin/findb"
	"github.com/keep94/toolbox/db"
	"github.com/keep94/toolbox/http_util"
	"html/template"
	"net/http"
	"strconv"
)

const (
	kUnreconciled = "unreconciled"
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
  <script src="/static/selectall.js"></script>
</head>
<body>
{{.LeftNav}}
<div class="main">
<h2>{{.Account.Name}}</h2>    
<a href="#" onclick="document.forms[0].edit_id.value=-1; document.forms[0].submit()">New Entry</a>&nbsp;
<a href="#" onclick="document.forms[0].edit_id.value=-2; document.forms[0].submit()">Normal View</a>
<br><br>
Balance: {{FormatUSD .Account.Balance}}&nbsp;&nbsp;&nbsp;&nbsp;Reconciled: {{FormatUSD .Account.RBalance}}
<br><br>
<form method="post">
<input type="hidden" name="xsrf" value="{{.Xsrf}}">
<input type="hidden" name="edit_id" value="">
{{if .Values}}
<input type="submit" value="Reconcile"><br>
  <input type="checkbox" onchange="selectAll(this, 'selectable')">
  <table>
    <tr>
      <td>Reconciled</td>
      <td>Date</td>
      <td>Category</td>
      <td>Name</td>
      <td>Amount</td>
    </tr>
{{with $top := .}}
  {{range .Values}}
    <tr class="lineitem">
      <td><input type="checkbox" name="id" class="selectable" value="{{.Id}}"></td>
      <td>{{FormatDate .Date}}</td>
      <td>{{$top.CatName .CatPayment}}</td>
      <td><a href="#" onclick="document.forms[0].edit_id.value={{.Id}}; document.forms[0].submit()">{{.Name}}</td>
      <td align=right>{{FormatUSD .Total}}</td>
    </tr>
    <tr>
      <td>{{if .CheckNo}}{{.CheckNo}}{{else}}&nbsp;{{end}}</td>
      <td colspan=4>{{.Desc}}</td>
    </tr>
  {{end}}
{{end}}
  </table>
<input type="submit" value="Reconcile">
{{else}}
No unreconciled entries.
{{end}}
</form>
</div>
</body>
</html>`
)

var (
	kTemplate *template.Template
)

type Store interface {
	findb.EntriesByAccountIdRunner
	findb.DoEntryChangesRunner
}

type Handler struct {
	Doer     db.Doer
	PageSize int
	LN       *common.LeftNav
	Global   *common.Global
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	session := common.GetUserSession(r)
	store := session.Store.(Store)
	cache := session.Cache
	acctId, _ := strconv.ParseInt(r.Form.Get("acctId"), 10, 64)
	if r.Method == "POST" {
		editId, _ := strconv.ParseInt(r.Form.Get("edit_id"), 10, 64)
		// Alter DB only if xsrf token is valid
		if common.VerifyXsrfToken(r, kUnreconciled) {
			reconciler := func(p *fin.Entry) bool {
				return p.Reconcile(acctId)
			}
			ids := r.Form["id"]
			updates := make(map[int64]fin.EntryUpdater, len(ids))
			for _, idStr := range ids {
				id, _ := strconv.ParseInt(idStr, 10, 64)
				updates[id] = reconciler
			}
			store.DoEntryChanges(nil, &findb.EntryChanges{Updates: updates})
		}
		if editId != 0 {
			entryLinker := &common.EntryLinker{
				URL: r.URL, Sel: common.SelectAccount(acctId)}
			accountLinker := common.AccountLinker{}
			if editId == -2 {
				http_util.Redirect(
					w,
					r,
					accountLinker.AccountLink(acctId).String())
			} else if editId == -1 {
				http_util.Redirect(w, r, entryLinker.NewEntryLink(acctId).String())
			} else {
				http_util.Redirect(w, r, entryLinker.EntryLink(editId).String())
			}
			return
		}
	}
	cds := categories.CatDetailStore{}
	entries := make([]fin.Entry, 0, h.PageSize)
	consumer := consume2.Slice(consume2.AppendTo(&entries), 0, h.PageSize)
	account := fin.Account{}
	err := h.Doer.Do(func(t db.Transaction) (err error) {
		cds, _ = cache.Get(t)
		return findb.UnreconciledEntries(t, store, acctId, &account, consumer)
	})
	if err == findb.NoSuchId {
		fmt.Fprintln(w, "No such account.")
		return
	}
	if err != nil {
		http_util.ReportError(w, "Error reading database.", err)
		return
	}
	leftnav := h.LN.Generate(w, r, common.SelectAccount(acctId))
	if leftnav == "" {
		return
	}
	http_util.WriteTemplate(
		w,
		kTemplate,
		&view{
			entries,
			common.CatDisplayer{CatDetailStore: cds},
			common.NewXsrfToken(r, kUnreconciled),
			&account,
			leftnav,
			h.Global})
}

type view struct {
	Values []fin.Entry
	common.CatDisplayer
	Xsrf    string
	Account *fin.Account
	LeftNav template.HTML
	Global  *common.Global
}

func init() {
	kTemplate = common.NewTemplate("unreconciled", kTemplateSpec)
}
