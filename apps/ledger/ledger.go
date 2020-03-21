package main

import (
  "bytes"
  "errors"
  "flag"
  "fmt"
  "github.com/gorilla/context"
  "github.com/keep94/appcommon/date_util"
  "github.com/keep94/appcommon/db"
  "github.com/keep94/appcommon/db/sqlite_db"
  "github.com/keep94/appcommon/http_util"
  "github.com/keep94/appcommon/lockout"
  "github.com/keep94/appcommon/logging"
  "github.com/keep94/appcommon/mailer"
  "github.com/keep94/finance/apps/ledger/ac"
  "github.com/keep94/finance/apps/ledger/account"
  "github.com/keep94/finance/apps/ledger/catedit"
  "github.com/keep94/finance/apps/ledger/chpasswd"
  "github.com/keep94/finance/apps/ledger/common"
  "github.com/keep94/finance/apps/ledger/export"
  "github.com/keep94/finance/apps/ledger/frame"
  "github.com/keep94/finance/apps/ledger/leftnav"
  "github.com/keep94/finance/apps/ledger/list"
  "github.com/keep94/finance/apps/ledger/login"
  "github.com/keep94/finance/apps/ledger/logout"
  "github.com/keep94/finance/apps/ledger/recurringlist"
  "github.com/keep94/finance/apps/ledger/recurringsingle"
  "github.com/keep94/finance/apps/ledger/report"
  "github.com/keep94/finance/apps/ledger/single"
  "github.com/keep94/finance/apps/ledger/static"
  "github.com/keep94/finance/apps/ledger/totals"
  "github.com/keep94/finance/apps/ledger/trends"
  "github.com/keep94/finance/apps/ledger/unreconciled"
  "github.com/keep94/finance/apps/ledger/unreviewed"
  "github.com/keep94/finance/apps/ledger/upload"
  "github.com/keep94/finance/fin"
  "github.com/keep94/finance/fin/autoimport"
  "github.com/keep94/finance/fin/autoimport/csv"
  "github.com/keep94/finance/fin/autoimport/qfx"
  "github.com/keep94/finance/fin/autoimport/qfx/qfxdb"
  qfxsqlite "github.com/keep94/finance/fin/autoimport/qfx/qfxdb/for_sqlite"
  csqlite "github.com/keep94/finance/fin/categories/categoriesdb/for_sqlite"
  "github.com/keep94/finance/fin/findb/for_sqlite"
  "github.com/keep94/gosqlite/sqlite"
  "github.com/keep94/ramstore"
  "github.com/keep94/weblogs"
  "gopkg.in/yaml.v2"
  "log"
  "net/http"
  "os"
)

const (
  kPageSize = 25
  // Set to the same thing as kXsrfTimeout in common/common.go
  kSessionTimeout = 900
)

var (
  fSSLCrt string
  fSSLKey string
  fPort string
  fDb string
  fIcon string
  fTitle string
  fGmailConfig string
  fLinks bool
  fPopularity bool
)

var (
  kDoer db.Doer
  kCatDetailCache *csqlite.Cache
  kStore for_sqlite.Store
  kUploaders map[string]autoimport.Loader
  kReadOnlyCatDetailCache csqlite.ReadOnlyCache
  kReadOnlyStore for_sqlite.ReadOnlyStore
  kReadOnlyUploaders map[string]autoimport.Loader
  kSessionStore = ramstore.NewRAMStore(kSessionTimeout)
  kClock date_util.SystemClock
)

var (
  kGmailConfig *gmailConfigType
  kMailer *mailer.Mailer
  kLockout *lockout.Lockout
)

func main() {
  flag.Parse()
  if fDb == "" {
    fmt.Println("Need to specify at least -db flag.")
    flag.Usage()
    return
  }
  setupDb(fDb)
  if fGmailConfig != "" {
    setupGmail(fGmailConfig)
  }
  mux := http.NewServeMux()
  http.HandleFunc("/", rootRedirect)
  http.Handle("/static/", http.StripPrefix("/static", static.New()))
  if fIcon != "" {
    err := http_util.AddStaticFromFile(
        http.DefaultServeMux, "/images/favicon.ico", fIcon)
    if err != nil {
      fmt.Printf("Icon file not found - %s\n", fIcon)
    }
  }
  if fGmailConfig != "" {
    http.Handle(
        "/auth/login",
        &login.Handler{
            Doer: kDoer,
            SessionStore: kSessionStore,
            Store: kStore,
            LO: kLockout,
            Mailer: kMailer,
            Recipients: kGmailConfig.To,
            PopularityOn: fPopularity})
  } else {
    http.Handle(
        "/auth/login",
        &login.Handler{
            Doer: kDoer,
            SessionStore: kSessionStore,
            Store: kStore,
            PopularityOn: fPopularity})
  }
  http.Handle(
      "/fin/", &authHandler{mux})
  mux.Handle(
      "/fin/list",
      &list.Handler{
         Store: kReadOnlyStore,
         Cdc: kReadOnlyCatDetailCache,
         PageSize: kPageSize,
         Links: fLinks})
  mux.Handle(
      "/fin/recurringlist",
      &recurringlist.Handler{
          Doer: kDoer, Cdc: kReadOnlyCatDetailCache, Clock: kClock})
  mux.Handle(
      "/fin/account",
      &account.Handler{
          Store: kReadOnlyStore,
          Cdc: kReadOnlyCatDetailCache,
          Doer: kDoer,
          PageSize: kPageSize,
          Links: fLinks})
  mux.Handle("/fin/single", &single.Handler{Doer: kDoer, Clock: kClock})
  mux.Handle(
      "/fin/recurringsingle",
      &recurringsingle.Handler{Doer: kDoer, Clock: kClock})
  mux.Handle("/fin/catedit", &catedit.Handler{})
  mux.Handle("/fin/logout", &logout.Handler{})
  // For now, the chpasswd handler gets full access to store
  mux.Handle("/fin/chpasswd", &chpasswd.Handler{Store: kStore, Doer: kDoer})
  mux.Handle(
      "/fin/leftnav",
      &leftnav.Handler{Cdc: kReadOnlyCatDetailCache, Clock: kClock})
  mux.Handle("/fin/frame", &frame.Handler{Title: fTitle})
  mux.Handle(
      "/fin/report",
      &report.Handler{Cdc: kReadOnlyCatDetailCache, Store: kReadOnlyStore})
  mux.Handle(
      "/fin/trends",
      &trends.Handler{Store: kReadOnlyStore, Cdc:kReadOnlyCatDetailCache})
  mux.Handle(
      "/fin/totals",
      &totals.Handler{Store: kReadOnlyStore})
  mux.Handle(
      "/fin/export",
      &export.Handler{
          Store: kReadOnlyStore,
          Cdc: kReadOnlyCatDetailCache,
          Clock: kClock})
  mux.Handle(
      "/fin/unreconciled",
      &unreconciled.Handler{Doer: kDoer, PageSize: kPageSize})
  mux.Handle(
      "/fin/unreviewed", &unreviewed.Handler{Doer: kDoer, PageSize: kPageSize})
  mux.Handle("/fin/upload", &upload.Handler{Doer: kDoer})
  mux.Handle(
      "/fin/acname",
      &ac.Handler{
          Store: kReadOnlyStore,
          Field: func (e *fin.Entry) string { return e.Name }})
  mux.Handle(
      "/fin/acdesc",
      &ac.Handler{
          Store: kReadOnlyStore,
          Field: func (e *fin.Entry) string { return e.Desc }})
  
  defaultHandler := context.ClearHandler(
      weblogs.HandlerWithOptions(
          http.DefaultServeMux,
          &weblogs.Options{Logger: logging.ApacheCommonLoggerWithLatency()}))
  if fSSLCrt != "" && fSSLKey != "" {
    if err := http.ListenAndServeTLS(fPort, fSSLCrt, fSSLKey, defaultHandler); err != nil {
      fmt.Println(err)
    }
    return
  }
  if err := http.ListenAndServe(fPort, defaultHandler); err != nil {
    fmt.Println(err)
  }
}  

type authHandler struct {
  *http.ServeMux
}

func (h *authHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  session, err := common.NewUserSession(kReadOnlyStore, kSessionStore, r)
  if err != nil {
    http_util.ReportError(w, "Error reading database.", err)
    return
  }
  if session.User == nil || !setupStores(session) {
    redirectString := r.URL.String()
    // Never have login page redirect to logout page.
    if redirectString == "/fin/logout" {
      redirectString = "/fin/frame"
    }
    http_util.Redirect(
        w,
        r,
        http_util.NewUrl("/auth/login", "prev", redirectString).String())
    return
  }
  logging.SetUserName(r, session.User.Name)
  h.ServeMux.ServeHTTP(w, r)
}

func rootRedirect(w http.ResponseWriter, r *http.Request) {
  if r.URL.Path == "/" {
    http_util.Redirect(w, r, "/fin/frame")
  } else {
    http_util.Error(w, http.StatusNotFound)
  }
}

func init() {
  flag.StringVar(&fSSLCrt, "ssl_crt", "", "SSL Certificate file")
  flag.StringVar(&fSSLKey, "ssl_key", "", "SSL Key file")
  flag.StringVar(&fPort, "http", ":8080", "Port to bind")
  flag.StringVar(&fDb, "db", "", "Path to database file")
  flag.StringVar(&fIcon, "icon", "", "Path to icon file")
  flag.StringVar(&fTitle, "title", "Finances", "Application title")
  flag.StringVar(&fGmailConfig, "gmail_config", "", "Gmail config file path")
  flag.BoolVar(&fLinks, "links", false, "Show categories as links in listings")
  flag.BoolVar(
      &fPopularity,
      "popularity",
      false,
      "Show most popular categories first in dropdowns")
}

func setupDb(filepath string) {
  conn, err := sqlite.Open(filepath)
  if err != nil {
    panic(err.Error())
  }
  dbase := sqlite_db.New(conn)
  qfxdata := qfxsqlite.New(dbase)
  kDoer = sqlite_db.NewDoer(dbase)
  kCatDetailCache = csqlite.New(dbase)
  kStore = for_sqlite.New(dbase)
  qfxLoader := qfx.QFXLoader{qfxdata}
  csvLoader := csv.CsvLoader{qfxdata}
  kUploaders = map[string]autoimport.Loader{
      ".qfx" : qfxLoader,
      ".ofx" : qfxLoader,
      ".csv" : csvLoader}
  kReadOnlyCatDetailCache = csqlite.ReadOnlyWrapper(kCatDetailCache)
  kReadOnlyStore = for_sqlite.ReadOnlyWrapper(kStore)
  readOnlyQFXLoader := qfx.QFXLoader{qfxdb.ReadOnlyWrapper(qfxdata)}
  readOnlyCsvLoader := csv.CsvLoader{qfxdb.ReadOnlyWrapper(qfxdata)}
  kReadOnlyUploaders = map[string]autoimport.Loader{
      ".qfx" : readOnlyQFXLoader,
      ".ofx" : readOnlyQFXLoader,
      ".csv" : readOnlyCsvLoader}
}

func setupStores(session *common.UserSession) bool {
  switch (session.User.Permission) {
    case fin.AllPermission:
      session.Store = kStore
      session.Cache = kCatDetailCache
      session.Uploaders = kUploaders
      return true
    case fin.ReadPermission:
      session.Store = kReadOnlyStore
      session.Cache = kReadOnlyCatDetailCache
      session.Uploaders = kReadOnlyUploaders
      return true
    default:
      return false
  }
}

type gmailConfigType struct {
  Email string `yaml:"email"`
  Password string `yaml:"password"`
  To []string `yaml:"to"`
  Failures int `yaml:"failures"`
}

func readGmailConfig(fileName string) (*gmailConfigType, error) {
  f, err := os.Open(fileName)
  if err != nil {
    return nil, err
  }
  defer f.Close()
  var content bytes.Buffer
  if _, err := content.ReadFrom(f); err != nil {
    return nil, err
  }
  var result gmailConfigType
  if err := yaml.Unmarshal(content.Bytes(), &result); err != nil {
    return nil, err
  }
  if result.Email == "" || result.Password == "" || len(result.To) == 0 || result.Failures < 1 {
    return nil, errors.New(
        "email, password, to, and failures fields required")
  }
  return &result, nil
}

func setupGmail(configPath string) {
  var err error
  kGmailConfig, err = readGmailConfig(configPath)
  if err != nil {
    log.Fatalf("Error reading config file: %v", err)
  }
  kMailer = mailer.New(kGmailConfig.Email, kGmailConfig.Password)
  kLockout = lockout.New(kGmailConfig.Failures)
}
