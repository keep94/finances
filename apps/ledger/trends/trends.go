package trends

import (
	"errors"
	"github.com/keep94/consume2"
	"github.com/keep94/finances/apps/ledger/common"
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/aggregators"
	"github.com/keep94/finances/fin/categories"
	"github.com/keep94/finances/fin/categories/categoriesdb"
	"github.com/keep94/finances/fin/consumers"
	"github.com/keep94/finances/fin/filters"
	"github.com/keep94/finances/fin/findb"
	"github.com/keep94/toolbox/date_util"
	"github.com/keep94/toolbox/google_jsgraph"
	"github.com/keep94/toolbox/http_util"
	"html/template"
	"net/http"
	"net/url"
	"time"
)

const (
	kPageParam        = "pageNo"
	kMaxPointsInGraph = 24
)

const (
	kExpenseColor = "660000"
	kIncomeColor  = "006600"
)

var (
	kTemplateSpec = `
{{define "MultiGraph"}}
<table>
  <tr>
    <td>
      <table border=1>
        <tr>
          <td>Date</td>
          <td>Income</td>
          <td>Expense</td>
          <td>Report</td>
        </tr>
{{with $top := .}}
{{range .MultiItems}}
        <tr>
  {{if .Url}}
          <td><a href="{{.Url}}">{{.Date.Format $top.FormatStr}}</a></td>
  {{else}}
          <td>{{.Date.Format $top.FormatStr}}</td>
  {{end}}
          <td align="right">{{FormatUSDRaw .IncomeValue}}</td>
          <td align="right">{{FormatUSDRaw .ExpenseValue}}</td>
  {{if .ReportUrl}}
          <td><a href="{{.ReportUrl}}">report</a></td>
  {{else}}
          <td>&nbsp;</td>
  {{end}}
        </tr>
{{end}}
{{end}}
      </table>
    </td>
    <td>
{{if .BarGraph}}
  <div id="graph" style="width: 500px; height: 300px;"></div>
{{else}}
  &nbsp;
{{end}}
    </td>
  </tr>
</table>
{{end}}
{{define "Graph"}}
<table>
  <tr>
    <td>
      <table border=1>
        <tr>
          <td>Date</td>
          <td>Amount</td>
          <td>Report</td>
        </tr>
{{with $top := .}}
{{range .Items}}
        <tr>
  {{if .Url}}
          <td><a href="{{.Url}}">{{.Date.Format $top.FormatStr}}</a></td>
  {{else}}
          <td>{{.Date.Format $top.FormatStr}}</td>
  {{end}}
        <td align="right">{{FormatUSDRaw .Value}}</td>
  {{if .ReportUrl}}
          <td><a href="{{.ReportUrl}}">report</a></td>
  {{else}}
          <td>&nbsp;</td>
  {{end}}
        </tr>
{{end}}
{{end}}
      </table>
    </td>
    <td>
{{if .BarGraph}}
  <div id="graph" style="width: 600px; height: 300px;"></div>
{{else}}
  &nbsp;
{{end}}
    </td>
  </tr>
</table>
{{end}}
<html>
  <head>
    <title>{{.Global.Title}}</title>
    {{if .Global.Icon}}
      <link rel="shortcut icon" href="/images/favicon.ico" type="image/x-icon" />
    {{end}}
    <link rel="stylesheet" type="text/css" href="/static/theme.css" />
    {{.GraphCode}}
  </head>
  <body>
  {{.LeftNav}}
  <div class="main">
{{if .Error}}
  <span class="error">{{.Error}}</span>
{{end}}
    <form method="get">
      <table>
        <tr>
          <td>Category: </td>
          <td>
            <select name="cat">
{{with .GetSelection .CatSelectModel "cat"}}
              <option value="{{.Value}}">{{.Name}}</option>
{{end}}
              <option value="">ALL</option>
{{range .CatDetails}}
              <option value="{{.Id}}">{{.FullName}}</option>
{{end}}
            </select>
          </td>
          <td>Start date: </td>
          <td><input type="text" name="sd" value="{{.Get "sd"}}"></td>
          <td>End date: </td>
          <td><input type="text" name="ed" value="{{.Get "ed"}}"></td>
        </tr>
        <tr>
          <td>Top level: </td>
          <td><input type="checkbox" name="top" {{if .Get "top"}}checked{{end}}></td>
          <td>Frequency: </td>
          <td colspan="3"><select name="freq">
            <option value="M" {{if .Equals "freq" "M"}}selected{{end}}>Monthly</option>
            <option value="Y" {{if .Equals "freq" "Y"}}selected{{end}}>Yearly</option>
          </select></td>
        </tr>
        <tr>
          <td colspan="6">
            <input type="submit" value="Generate report">
          </td>
        </tr>
      </table>
    </form>
{{if .Items}}
  {{template "Graph" .}}
{{else}}
  {{template "MultiGraph" .}}
{{end}}
  </div>
  </body>
</html>`
)

var (
	kTemplate *template.Template
)

type Handler struct {
	Cdc    categoriesdb.Getter
	Store  findb.EntriesRunner
	LN     *common.LeftNav
	Global *common.Global
	NoWifi bool
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	leftnav := h.LN.Generate(w, r, common.SelectTrends())
	if leftnav == "" {
		return
	}
	cds, _ := h.Cdc.Get(nil)
	cat, caterr := fin.CatFromString(r.Form.Get("cat"))
	start, end, err := getDateRange(r)
	if err != nil {
		v := &view{
			Values:       http_util.Values{Values: r.Form},
			CatDisplayer: common.CatDisplayer{CatDetailStore: cds},
			Error:        errors.New("Dates must be in yyyyMMdd format."),
			CatDetails:   cds.DetailsByIds(fin.CatSet{fin.Expense: true, fin.Income: true}),
			LeftNav:      leftnav,
			Global:       h.Global,
		}
		http_util.WriteTemplate(w, kTemplate, v)
		return
	}
	if caterr == nil {
		points, barGraph, cats, err := h.singleCat(cds, r.URL, cat, r.Form.Get("top") != "", start, end, r.Form.Get("freq") == "Y")
		if err != nil {
			http_util.ReportError(w, "Error reading database.", err)
			return
		}
		v := &view{
			Values:       http_util.Values{Values: r.Form},
			CatDisplayer: common.CatDisplayer{CatDetailStore: cds},
			Items:        points,
			CatDetails:   cds.DetailsByIds(cats),
			BarGraph:     barGraph,
			FormatStr:    formatStringLong(r.Form.Get("freq") == "Y"),
			LeftNav:      leftnav,
			GraphCode:    h.mustEmitGraphCode(barGraph),
			Global:       h.Global,
		}
		http_util.WriteTemplate(w, kTemplate, v)
	} else {
		points, barGraph, cats, err := h.allCats(cds, r.URL, start, end, r.Form.Get("freq") == "Y")
		if err != nil {
			http_util.ReportError(w, "Error reading database.", err)
			return
		}
		v := &view{
			Values:       http_util.Values{Values: r.Form},
			CatDisplayer: common.CatDisplayer{CatDetailStore: cds},
			MultiItems:   points,
			CatDetails:   cds.DetailsByIds(cats),
			BarGraph:     barGraph,
			FormatStr:    formatStringLong(r.Form.Get("freq") == "Y"),
			LeftNav:      leftnav,
			GraphCode:    h.mustEmitGraphCode(barGraph),
			Global:       h.Global,
		}
		http_util.WriteTemplate(w, kTemplate, v)
	}
}

func (h *Handler) singleCat(
	cds categories.CatDetailStore,
	thisUrl *url.URL,
	cat fin.Cat,
	topOnly bool,
	start, end time.Time,
	isYearly bool) (points []*dataPoint, barGraph *google_jsgraph.BarGraph, cats fin.CatSet, err error) {
	// Only to see what the child categories are
	ct := make(fin.CatTotals)
	totals := createByPeriodTotaler(start, end, isYearly)
	cr := consume2.Filterp(
		consume2.Compose(
			consumers.FromCatPaymentAggregator(ct),
			consumers.FromEntryAggregator(totals)),
		filters.CompileAdvanceSearchSpec(
			&filters.AdvanceSearchSpec{CF: cds.Filter(cat, !topOnly)}))
	elo := findb.EntryListOptions{
		Start: &start,
		End:   &end}
	err = h.Store.Entries(nil, &elo, cr)
	if err != nil {
		return
	}
	isIncome := cat.Type == fin.IncomeCat
	var listUrl *url.URL
	if topOnly {
		listUrl = http_util.NewUrl(
			"/fin/list",
			"cat", cat.String(),
			"top", "on")
	} else {
		listUrl = http_util.NewUrl(
			"/fin/list",
			"cat", cat.String())
	}
	var reportUrl *url.URL
	if isYearly {
		reportUrl = http_util.WithParams(thisUrl, "freq", "M")
	}
	builder := dataSetBuilder{
		ListUrl:   listUrl,
		ReportUrl: reportUrl,
		Totals:    totals,
		IsIncome:  isIncome}
	points = builder.Build()
	if len(points) <= kMaxPointsInGraph {
		g := &graphable{
			Data: points,
			Fmt:  formatString(isYearly)}
		if isIncome {
			barGraph = &google_jsgraph.BarGraph{
				Data:    g,
				Palette: []string{kIncomeColor},
			}
		} else {
			barGraph = &google_jsgraph.BarGraph{
				Data:    g,
				Palette: []string{kExpenseColor},
			}
		}
	}
	_, children := cds.RollUp(ct)
	cats = fin.CatSet{fin.Expense: true, fin.Income: true}
	cats.AddSet(children[cat])
	return
}

func (h *Handler) allCats(
	cds categories.CatDetailStore,
	thisUrl *url.URL,
	start, end time.Time,
	isYearly bool) (points []*multiDataPoint, barGraph *google_jsgraph.BarGraph, cats fin.CatSet, err error) {
	// Only to see what the child categories are
	ct := make(fin.CatTotals)
	expenseTotals := createByPeriodTotaler(start, end, isYearly)
	incomeTotals := createByPeriodTotaler(start, end, isYearly)
	cr := consume2.Compose(
		consumers.FromCatPaymentAggregator(ct),
		consume2.Filterp(
			consumers.FromEntryAggregator(expenseTotals),
			filters.CompileAdvanceSearchSpec(
				&filters.AdvanceSearchSpec{
					CF: cds.Filter(fin.Expense, true)})),
		consume2.Filterp(
			consumers.FromEntryAggregator(incomeTotals),
			filters.CompileAdvanceSearchSpec(
				&filters.AdvanceSearchSpec{
					CF: cds.Filter(fin.Income, true)})))
	elo := findb.EntryListOptions{
		Start: &start,
		End:   &end}
	err = h.Store.Entries(nil, &elo, cr)
	if err != nil {
		return
	}
	listUrl := http_util.NewUrl("/fin/list")
	var reportUrl *url.URL
	if isYearly {
		reportUrl = http_util.WithParams(thisUrl, "freq", "M")
	}
	builder := multiDataSetBuilder{
		ListUrl:       listUrl,
		ReportUrl:     reportUrl,
		ExpenseTotals: expenseTotals,
		IncomeTotals:  incomeTotals}
	points = builder.Build()
	if len(points) <= kMaxPointsInGraph {
		g := &multiGraphable{
			Data: points,
			Fmt:  formatString(isYearly)}
		barGraph = &google_jsgraph.BarGraph{
			Data:    g,
			Palette: []string{kIncomeColor, kExpenseColor},
		}
	}
	_, children := cds.RollUp(ct)
	cats = fin.CatSet{fin.Expense: true, fin.Income: true}
	cats.AddSet(children[fin.Expense]).AddSet(children[fin.Income])
	return
}

func (h *Handler) mustEmitGraphCode(
	barGraph *google_jsgraph.BarGraph) template.HTML {
	if h.NoWifi || barGraph == nil {
		return ""
	}
	graphMap := map[string]google_jsgraph.Graph{"graph": barGraph}
	return google_jsgraph.MustEmit(graphMap)
}

func formatString(isYearly bool) string {
	if isYearly {
		return "06"
	}
	return "01"
}

func formatStringLong(isYearly bool) string {
	if isYearly {
		return "2006"
	}
	return "01/2006"
}

func createByPeriodTotaler(start, end time.Time, isYearly bool) *aggregators.ByPeriodTotaler {
	if isYearly {
		return aggregators.NewByPeriodTotaler(start, end, aggregators.Yearly())
	}
	return aggregators.NewByPeriodTotaler(start, end, aggregators.Monthly())
}

func getDateRange(r *http.Request) (start, end time.Time, err error) {
	start, err = time.Parse(
		date_util.YMDFormat, common.NormalizeYMDStr(r.Form.Get("sd")))
	if err != nil {
		return
	}
	end, err = time.Parse(
		date_util.YMDFormat, common.NormalizeYMDStr(r.Form.Get("ed")))
	if err != nil {
		return
	}
	return
}

type dataPoint struct {
	Date      time.Time
	Value     int64
	Url       *url.URL
	ReportUrl *url.URL
}

type graphable struct {
	Data []*dataPoint
	Fmt  string
}

func (g *graphable) XLen() int { return len(g.Data) }

func (g *graphable) YLen() int { return 1 }

func (g *graphable) XLabel(i int) string {
	return g.Data[i].Date.Format(g.Fmt)
}

func (g *graphable) YLabel(i int) string { return "amount" }

func (g *graphable) XTitle() string { return "period" }

func (g *graphable) Value(x, y int) float64 {
	return max(float64(g.Data[x].Value)/100.0, 0.0)
}

type multiDataPoint struct {
	Date         time.Time
	IncomeValue  int64
	ExpenseValue int64
	Url          *url.URL
	ReportUrl    *url.URL
}

type multiGraphable struct {
	Data []*multiDataPoint
	Fmt  string
}

func (g *multiGraphable) XLen() int { return len(g.Data) }

func (g *multiGraphable) YLen() int { return 2 }

func (g *multiGraphable) XLabel(i int) string {
	return g.Data[i].Date.Format(g.Fmt)
}

func (g *multiGraphable) YLabel(y int) string {
	if y == 0 {
		return "Income"
	}
	return "Expense"
}

func (g *multiGraphable) XTitle() string { return "period" }

func (g *multiGraphable) Value(x, y int) float64 {
	var result float64
	if y == 0 {
		result = float64(g.Data[x].IncomeValue) / 100.0
	} else {
		result = float64(g.Data[x].ExpenseValue) / 100.0
	}
	return max(result, 0.0)
}

type view struct {
	http_util.Values
	common.CatDisplayer
	Items      []*dataPoint
	MultiItems []*multiDataPoint
	BarGraph   *google_jsgraph.BarGraph
	CatDetails []categories.CatDetail
	Error      error
	FormatStr  string
	LeftNav    template.HTML
	GraphCode  template.HTML
	Global     *common.Global
}

type dataSetBuilder struct {
	ListUrl   *url.URL
	ReportUrl *url.URL
	Totals    *aggregators.ByPeriodTotaler
	IsIncome  bool
}

func (b *dataSetBuilder) Build() (result []*dataPoint) {
	iter := b.Totals.Iterator()
	var pt aggregators.PeriodTotal
	for iter.Next(&pt) {
		if !b.IsIncome {
			pt.Total = -pt.Total
		}
		var reportUrl *url.URL
		sd := pt.Start.Format(date_util.YMDFormat)
		ed := pt.End.Format(date_util.YMDFormat)
		if b.ReportUrl != nil {
			reportUrl = http_util.WithParams(b.ReportUrl,
				"sd", sd,
				"ed", ed)
		}
		item := &dataPoint{
			Date:      pt.PeriodStart,
			Value:     pt.Total,
			ReportUrl: reportUrl,
			Url: http_util.WithParams(
				b.ListUrl,
				"sd", sd,
				"ed", ed)}
		result = append(result, item)
	}
	return result
}

type multiDataSetBuilder struct {
	ListUrl       *url.URL
	ReportUrl     *url.URL
	ExpenseTotals *aggregators.ByPeriodTotaler
	IncomeTotals  *aggregators.ByPeriodTotaler
}

func (b *multiDataSetBuilder) Build() (result []*multiDataPoint) {
	iter := b.IncomeTotals.Iterator()
	expenseIter := b.ExpenseTotals.Iterator()
	var pt, ept aggregators.PeriodTotal
	for iter.Next(&pt) {
		if !expenseIter.Next(&ept) {
			panic("expense totals shorter than income totals.")
		}
		var reportUrl *url.URL
		sd := pt.Start.Format(date_util.YMDFormat)
		ed := pt.End.Format(date_util.YMDFormat)
		if b.ReportUrl != nil {
			reportUrl = http_util.WithParams(b.ReportUrl,
				"sd", sd,
				"ed", ed)
		}
		item := &multiDataPoint{
			Date:         pt.PeriodStart,
			IncomeValue:  pt.Total,
			ExpenseValue: -ept.Total,
			ReportUrl:    reportUrl,
			Url: http_util.WithParams(
				b.ListUrl,
				"sd", sd,
				"ed", ed)}
		result = append(result, item)
	}
	return result
}

func init() {
	kTemplate = common.NewTemplate("trends", kTemplateSpec)
}
