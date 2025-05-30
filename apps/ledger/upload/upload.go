package upload

import (
	"bytes"
	"errors"
	"github.com/keep94/consume2"
	"github.com/keep94/finances/apps/ledger/common"
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/aggregators"
	"github.com/keep94/finances/fin/autoimport"
	"github.com/keep94/finances/fin/autoimport/reconcile"
	"github.com/keep94/finances/fin/consumers"
	"github.com/keep94/finances/fin/findb"
	"github.com/keep94/toolbox/date_util"
	"github.com/keep94/toolbox/db"
	"github.com/keep94/toolbox/http_util"
	"html/template"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	kUpload = "upload"
)

const (
	kMaxUploadSize          = 1024 * 1024
	kMaxDays                = 7
	kAutoCategorizeLookBack = 1000
)

var (
	kUploadTemplateSpec = `
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
<h2>{{.Account.Name}} Import Entries</h2>
{{if .Error}}
  <span class="error">{{.Error}}</span>
{{end}}
<form method="post" enctype="multipart/form-data">
  <input type="hidden" name="xsrf" value="{{.Xsrf}}">
  <table>
    <tr>
      <td>QFX file: </td>
      <td><input type="file" name="contents"></td>
    </tr>
    <tr>
      <td>Start Date (YYYYmmdd): </td>
      <td><input type="text" name="sd" value="{{.StartDate}}"></td>
    </tr>
  </table>
  <table>
    <tr>
      <td><input type="submit" name="upload" value="Upload"></td>
      <td><input type="submit" name="cancel" value="Cancel"></td>
    </tr>
  </table>
</form>
</div>
</body>
</html>`

	kConfirmTemplateSpec = `
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
<h2>{{.Account.Name}} Import Entries</h2>
<form method="post">
  <input type="hidden" name="task" value="confirm">
  <input type="hidden" name="xsrf" value="{{.Xsrf}}">
  <table>
    <tr>
      <td>New entries: </td>
      <td>{{.NewCount}}</td>
    </tr>
    <tr>
      <td>Existing entries: </td>
      <td>{{.ExistingCount}}</td>
    </tr>
    <tr>
      <td colspan=2>&nbsp;</td>
    </tr>
    <tr>
      <td>Balance: </td>
      <td>{{FormatUSD .Balance}}</td>
    </tr>
    <tr>
      <td>Reconciled Balance: </td>
      <td>{{FormatUSD .RBalance}}</td>
    </tr>
  </table>
  <table>
    <tr>
      <td><input type="submit" name="upload" value="Confirm"></td>
      <td><input type="submit" name="cancel" value="Cancel"></td>
    </tr>
  </table>
</form>
</div>
</body>
</html>`
)

var (
	kUploadTemplate  *template.Template
	kConfirmTemplate *template.Template
)

type Store interface {
	findb.DoEntryChangesRunner
	findb.EntriesByAccountIdRunner
	findb.UpdateAccountImportSDRunner
}

type Handler struct {
	Doer   db.Doer
	LN     *common.LeftNav
	Global *common.Global
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	session := common.GetUserSession(r)
	store := session.Store.(Store)
	acctId, _ := strconv.ParseInt(r.Form.Get("acctId"), 10, 64)
	userSession := common.GetUserSession(r)
	batch := userSession.Batch(acctId)
	if batch == nil {
		h.serveUploadPage(w, r, acctId, store, session.Uploaders)
	} else {
		h.serveConfirmPage(w, r, acctId, batch, store)
	}
}

func (h *Handler) serveConfirmPageGet(
	w http.ResponseWriter,
	r *http.Request,
	acctId int64,
	batch autoimport.Batch,
	store Store) {
	account := fin.Account{}
	var unreconciled []*fin.Entry
	err := h.Doer.Do(func(t db.Transaction) error {
		return findb.UnreconciledEntries(
			t,
			store,
			acctId,
			&account,
			consume2.AppendPtrsTo(&unreconciled))
	})
	if err != nil {
		http_util.ReportError(
			w, "A database error happened fetching unreconciled entries", err)
		return
	}
	batchEntries := batch.Entries()
	reconcile.Reconcile(unreconciled, kMaxDays, batchEntries)
	leftnav := h.LN.Generate(w, r, common.SelectAccount(acctId))
	if leftnav == "" {
		return
	}
	h.showConfirmView(
		w,
		computeConfirmView(&account, batchEntries),
		common.NewXsrfToken(r, kUpload),
		leftnav)
}

func (h *Handler) serveConfirmPage(w http.ResponseWriter, r *http.Request, acctId int64, batch autoimport.Batch, store Store) {
	if r.Method == "GET" {
		h.serveConfirmPageGet(w, r, acctId, batch, store)
	} else {
		// We are posting. If we are getting a post from the upload form
		// instead of the confirm form or the xsrf token is wrong,
		// then treat this as a GET
		if r.Form.Get("task") != "confirm" || !common.VerifyXsrfToken(r, kUpload) {
			h.serveConfirmPageGet(w, r, acctId, batch, store)
			return
		}
		if !http_util.HasParam(r.Form, "cancel") {
			categorizerBuilder := aggregators.NewByNameCategorizerBuilder(4, 2)
			// If this fails, we can carry on. We just won't get autocategorization
			store.Entries(
				nil,
				nil,
				consume2.Slice(
					consumers.FromEntryAggregator(categorizerBuilder),
					0,
					kAutoCategorizeLookBack),
			)
			categorizer := categorizerBuilder.Build()
			err := h.Doer.Do(func(t db.Transaction) (err error) {
				batch, err = batch.SkipProcessed(t)
				if err != nil {
					return
				}
				if batch.Len() == 0 {
					return
				}
				var unreconciled []*fin.Entry
				err = findb.UnreconciledEntries(
					t,
					store,
					acctId,
					nil,
					consume2.AppendPtrsTo(&unreconciled))
				if err != nil {
					return
				}
				batchEntries := batch.Entries()
				for i := range batchEntries {
					categorizer.Categorize(&batchEntries[i])
				}
				reconcile.Reconcile(unreconciled, kMaxDays, batchEntries)
				err = store.DoEntryChanges(
					t, reconcile.GetChanges(batchEntries))
				if err != nil {
					return
				}
				return batch.MarkProcessed(t)
			})
			if err != nil {
				http_util.ReportError(w, "A database error happened importing entries", err)
				return
			}
		}
		userSession := common.GetUserSession(r)
		userSession.SetBatch(acctId, nil)
		userSession.Save(r, w)
		accountLinker := common.AccountLinker{}
		http_util.Redirect(w, r, accountLinker.AccountLink(acctId).String())
	}
}

func (h *Handler) serverUploadPageGet(
	w http.ResponseWriter, r *http.Request, account *fin.Account) {
	leftnav := h.LN.Generate(w, r, common.SelectAccount(account.Id))
	if leftnav == "" {
		return
	}
	view := &view{
		Account:   account,
		StartDate: account.ImportSD.Format(date_util.YMDFormat),
		Xsrf:      common.NewXsrfToken(r, kUpload),
		LeftNav:   leftnav,
		Global:    h.Global}
	showView(w, view, nil)
}

func (h *Handler) serveUploadPage(
	w http.ResponseWriter, r *http.Request, acctId int64,
	store Store, uploaders map[string]autoimport.Loader) {
	account := fin.Account{}
	err := store.AccountById(nil, acctId, &account)
	if err != nil {
		http_util.ReportError(w, "Error reading account from database.", err)
		return
	}
	if r.Method == "GET" {
		h.serverUploadPageGet(w, r, &account)
	} else {
		reader, err := r.MultipartReader()
		if err != nil {
			// Assume we are getting a post from the confirm form instead
			// of the upload form. Treat as a GET.
			h.serverUploadPageGet(w, r, &account)
			return
		}
		mform, err := http_util.NewMultipartForm(
			reader, map[string]int{"contents": kMaxUploadSize})
		if err != nil {
			http_util.ReportError(w, "Error reading multipart form", err)
			return
		}
		if _, cancel := mform.GetFile("cancel"); cancel {
			accountLinker := common.AccountLinker{}
			http_util.Redirect(w, r, accountLinker.AccountLink(acctId).String())
			return
		}
		sdStr := mform.Get("sd")
		xsrf := mform.Get("xsrf")
		qfxFile, _ := mform.GetFile("contents")
		loader := uploaders[fileExtension(qfxFile.FileName)]
		leftnav := h.LN.Generate(w, r, common.SelectAccount(account.Id))
		if leftnav == "" {
			return
		}
		view := &view{
			Account:   &account,
			StartDate: sdStr,
			Xsrf:      common.NewXsrfToken(r, kUpload),
			LeftNav:   leftnav,
			Global:    h.Global}
		if !common.VerifyXsrfTokenExplicit(xsrf, r, kUpload) {
			showView(w, view, common.ErrXsrf)
			return
		}
		sd, err := time.Parse(date_util.YMDFormat, sdStr)
		if err != nil {
			showView(w, view, errors.New("Start date must be in yyyyMMdd format."))
			return
		}
		store.UpdateAccountImportSD(nil, acctId, sd)
		if len(qfxFile.Contents) >= kMaxUploadSize {
			showView(w, view, errors.New("File too large."))
			return
		}
		if len(qfxFile.Contents) == 0 {
			showView(w, view, errors.New("Please select a file."))
			return
		}
		if loader == nil {
			showView(w, view, errors.New("File extension not recognized."))
			return
		}
		batch, err := loader.Load(
			acctId, "", bytes.NewBuffer(qfxFile.Contents), sd)
		if err != nil {
			showView(w, view, err)
			return
		}
		batch, err = batch.SkipProcessed(nil)
		if err != nil {
			http_util.ReportError(w, "Error skipping already processed entries.", err)
			return
		}
		if batch.Len() == 0 {
			showView(w, view, errors.New("No new entries to process."))
			return
		}
		userSession := common.GetUserSession(r)
		userSession.SetBatch(acctId, batch)
		userSession.Save(r, w)
		http_util.Redirect(w, r, r.URL.String())
	}
}

func (h *Handler) showConfirmView(
	w http.ResponseWriter, v *confirmView, xsrf string, leftnav template.HTML) {
	v.Xsrf = xsrf
	v.LeftNav = leftnav
	v.Global = h.Global
	http_util.WriteTemplate(w, kConfirmTemplate, v)
}

func showView(
	w http.ResponseWriter, v *view, err error) {
	v.Error = err
	http_util.WriteTemplate(w, kUploadTemplate, v)
}

type view struct {
	Account   *fin.Account
	StartDate string
	Xsrf      string
	Error     error
	LeftNav   template.HTML
	Global    *common.Global
}

type confirmView struct {
	Account       *fin.Account
	NewCount      int
	ExistingCount int
	Balance       int64
	RBalance      int64
	Xsrf          string
	LeftNav       template.HTML
	Global        *common.Global
}

func computeConfirmView(
	account *fin.Account, batchEntries []fin.Entry) *confirmView {
	result := &confirmView{
		Account:  account,
		Balance:  account.Balance,
		RBalance: account.RBalance}
	for _, v := range batchEntries {
		total := v.Total()
		if v.Id == 0 {
			result.NewCount++
			result.Balance += total
		} else {
			result.ExistingCount++
		}
		result.RBalance += total
	}
	return result
}

func fileExtension(filename string) string {
	return strings.ToLower(path.Ext(filename))
}

func init() {
	kUploadTemplate = common.NewTemplate("upload", kUploadTemplateSpec)
	kConfirmTemplate = common.NewTemplate("upload_confirm", kConfirmTemplateSpec)
}
