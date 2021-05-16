package list

import (
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/keep94/consume"
	"github.com/keep94/finances/apps/ledger/common"
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/aggregators"
	"github.com/keep94/finances/fin/categories"
	"github.com/keep94/finances/fin/categories/categoriesdb"
	"github.com/keep94/finances/fin/consumers"
	"github.com/keep94/finances/fin/filters"
	"github.com/keep94/finances/fin/findb"
	"github.com/keep94/toolbox/date_util"
	"github.com/keep94/toolbox/http_util"
)

const (
	kPageParam = "pageNo"
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

<style type="text/css">
/*margin and padding on body element
  can introduce errors in determining
  element position and are not recommended;
  we turn them off as a foundation for YUI
  CSS treatments. */
body {
        margin:0;
        padding:0;
}
</style>

<!--CSS file (default YUI Sam Skin) -->
<link type="text/css" rel="stylesheet" href="/static/autocomplete.css">

<!-- Dependencies -->
<script src="/static/yahoo-dom-event.js"></script>
<script src="/static/datasource-min.js"></script>

<!-- Source file -->
<script src="/static/autocomplete-min.js"></script>

<script type="text/javascript" src="/static/json2.js"></script>
<script type="text/javascript" src="/static/ledger.js"></script>

<style type="text/css">
#nameAutoComplete {
  width:15em;
  padding-bottom:1em;
}

#descAutoComplete {
  width:15em;
  padding-bottom:1em;
}
</style>

</head>
<body class="yui-skin-sam">
{{.LeftNav}}
<div class="main">
{{with .ErrorMessage}}
  <span class="error">{{.}}</span>
{{end}}
<form>
  <table>
    <tr>
      <td>Category: </td>
      <td>
        <select name="cat" size=1>
{{with .GetSelection .CatSelectModel "cat"}}
          <option value="{{.Value}}">{{.Name}}</option>
{{end}}
          <option value="">ALL</option>
{{range .ActiveCatDetails false}}
          <option value="{{.Id}}">{{.FullName}}</option>
{{end}}
        </select>
        <br>
        Top level only: <input type="checkbox" name="top" {{if .Get "top"}}checked{{end}}>
      </td>
      <td valign="top">Account: </td>
      <td valign="top">
        <select name="acctId" size=1>
{{with .GetSelection .AccountSelectModel "acctId"}}
          <option value="{{.Value}}">{{.Name}}</option>
{{end}}
          <option value="">ALL</option>
{{range .ActiveAccountDetails}}
          <option value="{{.Id}}">{{.Name}}</option>
{{end}}
        </select>
      </td>
    </tr>
    <tr>
      <td>Start Date (yyyyMMdd): </td>
      <td><input type="text" name="sd" value="{{.Get "sd"}}"></td>
      <td>End Date (yyyyMMdd): </td>
      <td><input type="text" name="ed" value="{{.Get "ed"}}"></td>
    </tr>
    <tr>
      <td>Name: </td>
      <td>
        <div id="nameAutoComplete">
          <input type="text" id="nameField" name="name" value="{{.Get "name"}}">
          <div id="nameContainer"></div>
        </div>
      </td>
      <td>Range: </td>
      <td><input type="text" name="range" value="{{.Get "range"}}"></td>
    </tr>
    <tr>
      <td>Desc: </td>
      <td>
        <div id="descAutoComplete">
          <input type="desc" id="descField" name="desc" value="{{.Get "desc"}}">
          <div id="descContainer"></div>
        </div>
      </td>
    </tr>
  </table>
<input type="submit" value="Search">
</form>
<hr>
{{if .Totaler}}
<b>Total: {{FormatUSD .Total}}</b>&nbsp;&nbsp;
{{end}}
<a href="{{.NewEntryLink 0}}">New Entry</a>
<br><br>   
{{with $top := .}}
Page: {{.DisplayPageNo}}&nbsp;
{{if .PageNo}}<a href="{{.PrevPageLink}}">&lt;</a>{{end}}
{{if .End}}&nbsp;{{else}}<a href="{{.NextPageLink}}">&gt;</a>{{end}}
<br><br>
  <table>
    <tr>
      <td>Date</td>
      <td>Category</td>
      <td>Name</td>
      <td>Amount</td>
      <td>Account</td>
    </tr>
  {{range .Entries}}
      <tr class="lineitem">
        <td>{{FormatDate .Date}}</td>
        <td>{{range $top.CatLink .CatPayment}}{{if .Link}}<a href="{{.Link}}">{{.Text}}</a>{{else}}{{.Text}}{{end}}{{end}}</td>
        <td><a href="{{$top.EntryLink .Id}}">{{.Name}}</a></td>
        <td align=right>{{FormatUSD .Total}}</td>
        <td>{{with $top.AccountNameLink .CatPayment}}{{if .Link}}<a href="{{.Link}}">{{.Text}}</a>{{else}}{{.Text}}{{end}}{{end}}</td>
      </tr>
      <tr>
        <td>
          {{if .CheckNo}}{{.CheckNo}}{{else}}&nbsp;{{end}}
        </td>
        <td colspan=4>{{.Desc}}</td>
      </tr>
  {{end}}
  </table>
  <br>
Page: {{.DisplayPageNo}}&nbsp;
{{if .PageNo}}<a href="{{.PrevPageLink}}">&lt;</a>{{end}}
{{if .End}}&nbsp;{{else}}<a href="{{.NextPageLink}}">&gt;</a>{{end}}
</div>
<script type="text/javascript">
  var nameSuggester = new Suggester("/fin/acname");
  var descSuggester = new Suggester("/fin/acdesc");
  var nameDs = new YAHOO.util.FunctionDataSource(function() {
    return nameSuggester.getSuggestions();
  });
  var descDs = new YAHOO.util.FunctionDataSource(function() {
    return descSuggester.getSuggestions();
  });
  var nameAutoComplete = new YAHOO.widget.AutoComplete("nameField", "nameContainer", nameDs);
  initAutoComplete(nameAutoComplete);
  var descAutoComplete = new YAHOO.widget.AutoComplete("descField", "descContainer", descDs);
  initAutoComplete(descAutoComplete);
</script>
</body>
</html>
{{end}}`
)

var (
	kTemplate *template.Template
)

type Handler struct {
	Cdc      categoriesdb.Getter
	Store    findb.EntriesRunner
	PageSize int
	Links    bool
	LN       *common.LeftNav
	Global   *common.Global
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	selecter := common.SelectSearch()
	leftnav := h.LN.Generate(w, r, selecter)
	if leftnav == "" {
		return
	}
	pageNo, _ := strconv.Atoi(r.Form.Get(kPageParam))
	if pageNo < 0 {
		pageNo = 0
	}
	cds, _ := h.Cdc.Get(nil)
	var creater creater
	filter := creater.CreateFilterer(r.Form, cds)
	elo := creater.CreateEntryListOptions(r.Form)

	var totaler *aggregators.Totaler
	var entries []fin.Entry
	var morePages bool
	epb := consume.Page(pageNo, h.PageSize, &entries, &morePages)
	if filter != nil && elo.Start != nil {
		totaler = &aggregators.Totaler{}
	}
	err := h.Store.Entries(nil, elo, buildConsumer(epb, filter, totaler))
	epb.Finalize()
	if err != nil {
		http_util.ReportError(w, "Error reading database.", err)
		return
	}
	var listEntriesUrl *url.URL
	if h.Links {
		listEntriesUrl = r.URL
	}
	http_util.WriteTemplate(
		w,
		kTemplate,
		&view{
			http_util.PageBreadCrumb{
				URL:         r.URL,
				PageNoParam: kPageParam,
				PageNo:      pageNo,
				End:         !morePages},
			entries,
			totaler,
			http_util.Values{r.Form},
			common.CatDisplayer{cds},
			common.CatLinker{ListEntries: listEntriesUrl, Cds: cds},
			common.EntryLinker{URL: r.URL, Sel: selecter},
			creater.ErrorMessage,
			leftnav,
			h.Global})
}

func buildConsumer(
	consumer consume.Consumer,
	filter consume.MapFilterer,
	totaler *aggregators.Totaler) consume.Consumer {
	if totaler != nil {
		consumer = consume.Compose(
			consumers.FromCatPaymentAggregator(totaler),
			consumer)
	}
	if filter != nil {
		consumer = consume.MapFilter(consumer, filter)
	}
	return consumer
}

type creater struct {
	ErrorMessage string
}

func (c *creater) CreateEntryListOptions(
	values url.Values) *findb.EntryListOptions {
	sdPtr, sderr := getDateRelaxed(values, "sd")
	edPtr, ederr := getDateRelaxed(values, "ed")
	if sderr != nil || ederr != nil {
		c.ErrorMessage = "Start and end date must be in yyyyMMdd format."
		return &findb.EntryListOptions{}
	}
	return &findb.EntryListOptions{Start: sdPtr, End: edPtr}
}

func (c *creater) CreateFilterer(
	values url.Values, cds categories.CatDetailStore) consume.MapFilterer {
	filt := createCatFilter(values, cds)
	accountId, _ := strconv.ParseInt(values.Get("acctId"), 10, 64)
	amtFilter := c.createAmountFilter(values.Get("range"))
	name := values.Get("name")
	desc := values.Get("desc")
	if amtFilter != nil || filt != nil || accountId != 0 || name != "" || desc != "" {
		return filters.CompileAdvanceSearchSpec(&filters.AdvanceSearchSpec{
			CF:        filt,
			AF:        amtFilter,
			AccountId: accountId,
			Name:      name,
			Desc:      desc})
	}
	return nil
}

func (c *creater) createAmountFilter(rangeStr string) filters.AmountFilter {
	if rangeStr == "" {
		return nil
	}
	filter := compileRangeFilter(rangeStr)
	if filter == nil {
		c.ErrorMessage = "Range must be of form 12.34 to 56.78."
	}
	return filter
}

func createCatFilter(
	values url.Values, cds categories.CatDetailStore) fin.CatFilter {
	cat, caterr := fin.CatFromString(values.Get("cat"))
	if caterr != nil {
		return nil
	}
	return cds.Filter(cat, values.Get("top") == "")
}

type view struct {
	http_util.PageBreadCrumb
	Entries []fin.Entry
	*aggregators.Totaler
	http_util.Values
	common.CatDisplayer
	common.CatLinker
	common.EntryLinker
	ErrorMessage string
	LeftNav      template.HTML
	Global       *common.Global
}

func getDateRelaxed(values url.Values, key string) (*time.Time, error) {
	s := strings.TrimSpace(values.Get(key))
	if s == "" {
		return nil, nil
	}
	t, e := time.Parse(date_util.YMDFormat, common.NormalizeYMDStr(s))
	if e != nil {
		return nil, e
	}
	return &t, nil
}

func compileRangeFilter(expr string) filters.AmountFilter {
	expr = strings.ToLower(expr)
	parts := strings.SplitN(expr, "to", 2)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) == 1 {
		neededAmount, err := fin.ParseUSD(parts[0])
		if err != nil {
			return nil
		}
		return func(amt int64) bool {
			return amt == -neededAmount
		}
	}
	if parts[0] != "" && parts[1] != "" {
		lower, err := fin.ParseUSD(parts[0])
		if err != nil {
			return nil
		}
		upper, err := fin.ParseUSD(parts[1])
		if err != nil {
			return nil
		}
		return func(amt int64) bool {
			return amt >= -upper && amt <= -lower
		}
	}
	if parts[0] != "" {
		lower, err := fin.ParseUSD(parts[0])
		if err != nil {
			return nil
		}
		return func(amt int64) bool {
			return amt <= -lower
		}
	}
	if parts[1] != "" {
		upper, err := fin.ParseUSD(parts[1])
		if err != nil {
			return nil
		}
		return func(amt int64) bool {
			return amt >= -upper
		}
	}
	return nil
}

func init() {
	kTemplate = common.NewTemplate("list", kTemplateSpec)
}
