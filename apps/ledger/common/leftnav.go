package common

import (
	"errors"
	"fmt"
	"github.com/keep94/finances/fin/categories"
	"github.com/keep94/finances/fin/categories/categoriesdb"
	"github.com/keep94/toolbox/date_util"
	"github.com/keep94/toolbox/http_util"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var (
	kLeftNavTemplateSpec = `
<div class="leftnav">
{{with .BuildId}}
    <b>Build {{.}}</b><br><br>
{{end}}
<b>{{.UserName}}</b><br>
{{.LastLogin}}<br>
<br>
Accounts:
<ul>
{{with $top := .}}
  {{range .ActiveAccountDetails}}
    <li><a {{if $top.Account .Id}}class="selected"{{end}} href="{{$top.AccountLink .Id}}">{{.Name}}</a></li>
  {{end}}
{{end}}
</ul>
<br>
<a {{if .Reports}}class="selected"{{end}} href="{{.ReportUrl}}">Reports</a><br>
<a {{if .Trends}}class="selected"{{end}} href="{{.TrendUrl}}">Trends</a><br>
<a {{if .Totals}}class="selected"{{end}} href="/fin/totals">Totals</a><br>
<a {{if .Envelopes}}class="selected"{{end}} href="{{.EnvelopeUrl}}">Envelopes</a><br>
<br>
<a {{if .Search}}class="selected"{{end}} href="/fin/list">Search</a><br>
<a {{if .Unreviewed}}class="selected"{{end}} href="/fin/unreviewed">Review</a><br>
<a {{if .Manage}}class="selected"{{end}} href="/fin/catedit">Manage Categories</a><br>
<a {{if .Recurring}}class="selected"{{end}} href="/fin/recurringlist">Recurring</a><br>
<a {{if .Export}}class="selected"{{end}} href="/fin/export">Export</a><br>
<br>
<a {{if .Chpasswd}}class="selected"{{end}} href="/fin/chpasswd">Change Password</a><br>
<a href="/fin/logout">Sign out</a>
<br><br>
</div>`
)

var (
	kLeftNavTemplate *template.Template
)

// Selecter indicates the item to be selected in the left navigation bar.
type Selecter struct {
	cat int
	id  int64
}

func ParseSelecter(s string) (Selecter, error) {
	ss := strings.SplitN(s, ":", 2)
	if len(ss) < 2 {
		return Selecter{}, errors.New("common: Missing colon.")
	}
	cat, err := strconv.ParseInt(ss[0], 10, 64)
	if err != nil {
		return Selecter{}, err
	}
	id, err := strconv.ParseInt(ss[1], 10, 64)
	if err != nil {
		return Selecter{}, err
	}
	return Selecter{cat: int(cat), id: id}, nil
}

func (s Selecter) String() string {
	return fmt.Sprintf("%d:%d", s.cat, s.id)
}

const (
	accounts = iota + 1
	reports
	trends
	totals
	search
	unreviewed
	manage
	recurring
	export
	chpasswd
	envelopes
)

func SelectAccount(id int64) Selecter { return Selecter{cat: accounts, id: id} }
func SelectReports() Selecter         { return Selecter{cat: reports} }
func SelectTrends() Selecter          { return Selecter{cat: trends} }
func SelectTotals() Selecter          { return Selecter{cat: totals} }
func SelectSearch() Selecter          { return Selecter{cat: search} }
func SelectUnreviewed() Selecter      { return Selecter{cat: unreviewed} }
func SelectManage() Selecter          { return Selecter{cat: manage} }
func SelectRecurring() Selecter       { return Selecter{cat: recurring} }
func SelectExport() Selecter          { return Selecter{cat: export} }
func SelectChpasswd() Selecter        { return Selecter{cat: chpasswd} }
func SelectEnvelopes() Selecter       { return Selecter{cat: envelopes} }
func SelectNone() Selecter            { return Selecter{} }

// LeftNav is for creating the left navigation bar.
type LeftNav struct {
	Cdc     categoriesdb.Getter
	Clock   date_util.Clock
	BuildId string
}

// Generate generates the html for the left navigation bar including the div
// tags. sel indicates which item in the left navigation bar will be selected.
// If Generate can't generate the html, it returns the empty string, writes
// an error message to w, and writes the error to stderr.
func (l *LeftNav) Generate(
	w http.ResponseWriter, r *http.Request, sel Selecter) template.HTML {
	session := GetUserSession(r)
	lastLoginStr := "--"
	lastLogin, ok := session.LastLogin()
	if ok {
		lastLoginStr = lastLogin.Local().Format("Mon 01/02/2006 15:04")
	}
	cds, err := l.Cdc.Get(nil)
	if err != nil {
		http_util.ReportError(w, "Database error", err)
		return ""
	}
	now := date_util.TimeToDate(l.Clock.Now())
	currentYear := now.Year()
	// Include today!
	now = now.AddDate(0, 0, 1)
	oneMonthAgo := now.AddDate(0, -1, 0)
	oneYearAgo := now.AddDate(-1, 0, 0)
	var sb strings.Builder
	http_util.WriteTemplate(&sb, kLeftNavTemplate, &view{
		CatDetailStore: cds,
		BuildId:        l.BuildId,
		ReportUrl: http_util.NewUrl(
			"/fin/report",
			"sd", oneMonthAgo.Format(date_util.YMDFormat),
			"ed", now.Format(date_util.YMDFormat)),
		TrendUrl: http_util.NewUrl(
			"/fin/trends",
			"sd", oneYearAgo.Format(date_util.YMDFormat),
			"ed", now.Format(date_util.YMDFormat)),
		EnvelopeUrl: http_util.NewUrl(
			"/fin/envelopes",
			"year", strconv.Itoa(currentYear)),
		UserName:  session.User.Name,
		LastLogin: lastLoginStr,
		sel:       sel})
	return template.HTML(sb.String())
}

type view struct {
	AccountLinker
	categories.CatDetailStore
	BuildId     string
	ReportUrl   *url.URL
	TrendUrl    *url.URL
	EnvelopeUrl *url.URL
	UserName    string
	LastLogin   string
	sel         Selecter
}

func (v *view) Account(id int64) bool { return v.sel == SelectAccount(id) }
func (v *view) Reports() bool         { return v.sel == SelectReports() }
func (v *view) Trends() bool          { return v.sel == SelectTrends() }
func (v *view) Totals() bool          { return v.sel == SelectTotals() }
func (v *view) Search() bool          { return v.sel == SelectSearch() }
func (v *view) Unreviewed() bool      { return v.sel == SelectUnreviewed() }
func (v *view) Manage() bool          { return v.sel == SelectManage() }
func (v *view) Recurring() bool       { return v.sel == SelectRecurring() }
func (v *view) Export() bool          { return v.sel == SelectExport() }
func (v *view) Chpasswd() bool        { return v.sel == SelectChpasswd() }
func (v *view) Envelopes() bool       { return v.sel == SelectEnvelopes() }

func init() {
	kLeftNavTemplate = NewTemplate("leftnav", kLeftNavTemplateSpec)
}
