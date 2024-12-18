package report

import (
	"errors"
	"fmt"
	"github.com/keep94/finances/apps/ledger/common"
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/categories"
	"github.com/keep94/finances/fin/categories/categoriesdb"
	"github.com/keep94/finances/fin/consumers"
	"github.com/keep94/finances/fin/findb"
	"github.com/keep94/toolbox/date_util"
	"github.com/keep94/toolbox/google_jsgraph"
	"github.com/keep94/toolbox/http_util"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"time"
)

const (
	kPageParam        = "pageNo"
	kMaxPointsInGraph = 14
)

var (
	kPieGraphColors = []string{
		"000066", "666600", "660000", "006600", "660066",
		"006666", "333333", "6666CC", "CCCC66", "CC6666",
		"66CC66", "CC66CC", "66CCCC", "999999"}
)

var (
	kTemplateSpec = `
{{define "Graph"}}
<table>
  <tr>
    <td>
      <table border=1>
        <tr>
          <td>Category</td>
          <td>Amount</td>
          <td>Report</td>
        </tr>
{{range .Items}}
        <tr>
  {{if .Url}}
          <td><a href="{{.Url}}">{{.Name}}</a></td>
  {{else}}
          <td>{{.Name}}</td>
  {{end}}
        <td align="right">{{FormatUSDRaw .Value}}</td>
  {{if .ReportUrl}}
          <td><a href="{{.ReportUrl}}">report</a></td>
  {{else}}
          <td>&nbsp;</td>
  {{end}}
        </tr>
{{end}}
        <tr>
{{if .Url}}
          <td><b><a href="{{.Url}}">Total</a></b></td>
{{else}}
          <td><b>Total</b></td>
{{end}}
          <td align="right"><b>{{FormatUSDRaw .Total}}</b></td>
          <td>&nbsp;</td>
        </tr>
      </table>
    </td>
    <td>
{{if .HasGraph}}
  <div id="{{.DivName}}" style="width: 600px; height: 600px;"></div>
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
          <td colspan="6">
            <input type="submit" value="Generate report">
          </td>
        </tr>
      </table>
    </form>
{{if .Sets}}
  {{range .Sets}}
    {{if .Url}}
      <h2><a href="{{.Url}}">{{.Name}}</a></h2>
    {{else}}
      <h2>{{.Name}}</h2>
    {{end}}
    {{template "Graph" .}}
  {{end}}
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
	leftnav := h.LN.Generate(w, r, common.SelectReports())
	if leftnav == "" {
		return
	}
	cds, _ := h.Cdc.Get(nil)
	start, end, err := getDateRange(r)
	if err != nil {
		v := &view{
			Values:       http_util.Values{Values: r.Form},
			CatDisplayer: common.CatDisplayer{CatDetailStore: cds},
			CatDetails:   cds.DetailsByIds(fin.CatSet{fin.Expense: true, fin.Income: true}),
			Error:        errors.New("Dates must be in yyyyMMdd format."),
			LeftNav:      leftnav,
			Global:       h.Global}
		http_util.WriteTemplate(w, kTemplate, v)
		return
	}
	cat, caterr := fin.CatFromString(r.Form.Get("cat"))
	ct := make(fin.CatTotals)
	erc := consumers.FromCatPaymentAggregator(ct)
	elo := findb.EntryListOptions{Start: &start, End: &end}
	err = h.Store.Entries(nil, &elo, erc)
	if err != nil {
		http_util.ReportError(w, "Error reading database.", err)
		return
	}
	rolledCt, children := cds.RollUp(ct)
	builder := dataSetBuilder{
		ListUrl: http_util.NewUrl(
			"/fin/list",
			"sd", r.Form.Get("sd"),
			"ed", r.Form.Get("ed")),
		ReportUrl: r.URL,
		Cds:       cds,
		Unrolled:  ct,
		Totals:    rolledCt,
		Children:  children,
		Colors:    kPieGraphColors}
	catsInDropDown := fin.CatSet{fin.Expense: true, fin.Income: true}
	var displaySets []*dataSet
	if caterr == nil {
		displaySets = []*dataSet{builder.Build(cat, "expenses")}
		catsInDropDown.AddSet(children[cat])
	} else {
		displaySets = []*dataSet{builder.Build(fin.Expense, "expenses"), builder.Build(fin.Income, "income")}
		catsInDropDown.AddSet(children[fin.Expense]).AddSet(children[fin.Income])
	}
	v := &view{
		Values:       http_util.Values{Values: r.Form},
		CatDisplayer: common.CatDisplayer{CatDetailStore: cds},
		Sets:         displaySets,
		CatDetails:   cds.DetailsByIds(catsInDropDown),
		LeftNav:      leftnav,
		GraphCode:    h.mustEmitGraphCode(displaySets),
		Global:       h.Global}

	http_util.WriteTemplate(w, kTemplate, v)
}

func (h *Handler) mustEmitGraphCode(sets []*dataSet) template.HTML {
	if h.NoWifi {
		return ""
	}
	graphs := make(map[string]google_jsgraph.Graph, len(sets))
	for _, s := range sets {
		s.RegisterGraph(graphs)
	}
	return google_jsgraph.MustEmit(graphs)
}

type dataPoint struct {
	Name      string
	Value     int64
	Url       *url.URL
	ReportUrl *url.URL
}

type graphable []*dataPoint

func (g graphable) XLen() int           { return len(g) }
func (g graphable) YLen() int           { return 1 }
func (g graphable) XLabel(i int) string { return g[i].Name }
func (g graphable) YLabel(i int) string { return "amount" }
func (g graphable) XTitle() string      { return "category" }
func (g graphable) Value(x, y int) float64 {
	return float64(g[x].Value) / 100.0
}

type dataSet struct {
	Name       string
	Url        *url.URL
	Total      int64
	Items      []*dataPoint
	GraphItems graphable
	Colors     []string
	DivName    string
}

func (d *dataSet) HasGraph() bool {
	return d.GraphItems.XLen() > 0
}

func (d *dataSet) RegisterGraph(graphs map[string]google_jsgraph.Graph) {
	if d.HasGraph() {
		graphs[d.DivName] = &google_jsgraph.PieGraph{
			Data:    d.GraphItems,
			Palette: d.Colors,
		}
	}
}

type view struct {
	http_util.Values
	common.CatDisplayer
	Sets       []*dataSet
	CatDetails []categories.CatDetail
	Error      error
	LeftNav    template.HTML
	GraphCode  template.HTML
	Global     *common.Global
}

type dataSetBuilder struct {
	ListUrl   *url.URL
	ReportUrl *url.URL
	Cds       categories.CatDetailStore
	Unrolled  fin.CatTotals
	Totals    fin.CatTotals
	Children  map[fin.Cat]fin.CatSet
	Colors    []string
}

func (b *dataSetBuilder) Build(cat fin.Cat, divName string) *dataSet {
	childCats := b.Children[cat]
	childCatLength := len(childCats)
	result := &dataSet{
		Name:    b.Cds.DetailById(cat).FullName(),
		Url:     http_util.WithParams(b.ListUrl, "cat", cat.String()),
		Items:   make([]*dataPoint, childCatLength+1),
		Colors:  b.Colors,
		DivName: divName}
	isIncome := cat.Type == fin.IncomeCat
	idx := 0
	for childCat, ok := range childCats {
		if ok {
			item := &dataPoint{
				Name: b.Cds.DetailById(childCat).FullName(),
				Url:  http_util.WithParams(b.ListUrl, "cat", childCat.String())}
			if isIncome {
				item.Value = -b.Totals[childCat]
			} else {
				item.Value = b.Totals[childCat]
			}
			if _, ok := b.Children[childCat]; ok {
				item.ReportUrl = http_util.WithParams(
					b.ReportUrl, "cat", childCat.String())
			}
			result.Items[idx] = item
			idx++
		}
	}
	if isIncome {
		result.Total = -b.Totals[cat]
	} else {
		result.Total = b.Totals[cat]
	}
	if leftOver, ok := b.Unrolled[cat]; ok {
		uncategorizedItem := &dataPoint{
			Name: fmt.Sprintf("%s:uncategorized", result.Name),
			Url: http_util.WithParams(
				b.ListUrl, "cat", cat.String(), "top", "true")}
		if isIncome {
			uncategorizedItem.Value = -leftOver
		} else {
			uncategorizedItem.Value = leftOver
		}
		result.Items[idx] = uncategorizedItem
		idx++
	}
	result.Items = result.Items[:idx]
	sort.Sort(byAmount(result.Items))
	numPointsInGraph := sort.Search(len(result.Items), func(x int) bool { return result.Items[x].Value <= 0 })
	if numPointsInGraph > kMaxPointsInGraph {
		numPointsInGraph = kMaxPointsInGraph
	}
	result.GraphItems = graphable(result.Items[:numPointsInGraph])
	return result
}

type byAmount []*dataPoint

func (a byAmount) Len() int           { return len(a) }
func (a byAmount) Less(i, j int) bool { return a[i].Value > a[j].Value }
func (a byAmount) Swap(i, j int)      { a[j], a[i] = a[i], a[j] }

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

func init() {
	kTemplate = common.NewTemplate("report", kTemplateSpec)
}
