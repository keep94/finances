package main

import (
	"bytes"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/keep94/context"
	"github.com/keep94/finances/apps/ledger/ac"
	"github.com/keep94/finances/apps/ledger/account"
	"github.com/keep94/finances/apps/ledger/addenvelope"
	"github.com/keep94/finances/apps/ledger/catedit"
	"github.com/keep94/finances/apps/ledger/chpasswd"
	"github.com/keep94/finances/apps/ledger/common"
	"github.com/keep94/finances/apps/ledger/envelopes"
	"github.com/keep94/finances/apps/ledger/export"
	"github.com/keep94/finances/apps/ledger/list"
	"github.com/keep94/finances/apps/ledger/login"
	"github.com/keep94/finances/apps/ledger/logout"
	"github.com/keep94/finances/apps/ledger/recurringlist"
	"github.com/keep94/finances/apps/ledger/recurringsingle"
	"github.com/keep94/finances/apps/ledger/report"
	"github.com/keep94/finances/apps/ledger/single"
	"github.com/keep94/finances/apps/ledger/static"
	"github.com/keep94/finances/apps/ledger/totals"
	"github.com/keep94/finances/apps/ledger/trends"
	"github.com/keep94/finances/apps/ledger/unreconciled"
	"github.com/keep94/finances/apps/ledger/unreviewed"
	"github.com/keep94/finances/apps/ledger/upload"
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/autoimport"
	"github.com/keep94/finances/fin/autoimport/csv"
	"github.com/keep94/finances/fin/autoimport/qfx"
	"github.com/keep94/finances/fin/autoimport/qfx/qfxdb"
	qfxsqlite "github.com/keep94/finances/fin/autoimport/qfx/qfxdb/for_sqlite"
	csqlite "github.com/keep94/finances/fin/categories/categoriesdb/for_sqlite"
	"github.com/keep94/finances/fin/findb/for_sqlite"
	"github.com/keep94/ramstore"
	"github.com/keep94/toolbox/build"
	"github.com/keep94/toolbox/date_util"
	"github.com/keep94/toolbox/db"
	"github.com/keep94/toolbox/db/sqlite3_db"
	"github.com/keep94/toolbox/http_util"
	"github.com/keep94/toolbox/lockout"
	"github.com/keep94/toolbox/logging"
	"github.com/keep94/toolbox/mailer"
	"github.com/keep94/weblogs"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v2"
)

const (
	kPageSize = 25
	// Set to the same thing as kXsrfTimeout in common/common.go
	kSessionTimeout = 900
)

var (
	fSSLCrt             string
	fSSLKey             string
	fPort               string
	fDb                 string
	fIcon               string
	fTitle              string
	fGmailConfig        string
	fLinks              bool
	fPopularityLookback int
	fNoWifi             bool
)

var (
	kDoer                   db.Doer
	kCatDetailCache         *csqlite.Cache
	kStore                  for_sqlite.Store
	kUploaders              map[string]autoimport.Loader
	kReadOnlyCatDetailCache csqlite.ReadOnlyCache
	kReadOnlyStore          for_sqlite.ReadOnlyStore
	kReadOnlyUploaders      map[string]autoimport.Loader
	kSessionStore           = ramstore.NewRAMStore(kSessionTimeout)
	kClock                  date_util.SystemClock
)

var (
	kLockout           *lockout.Lockout
	kMailer            login.Sender
	kLockoutRecipients []string
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
	var hasIcon bool
	if fIcon != "" {
		err := http_util.AddStaticFromFile(
			http.DefaultServeMux, "/images/favicon.ico", fIcon)
		if err != nil {
			fmt.Printf("Icon file not found - %s\n", fIcon)
		} else {
			hasIcon = true
		}
	}
	global := &common.Global{Title: fTitle, Icon: hasIcon}
	http.Handle(
		"/auth/login",
		&login.Handler{
			Doer:               kDoer,
			SessionStore:       kSessionStore,
			Store:              kStore,
			LO:                 kLockout,
			Mailer:             kMailer,
			Recipients:         kLockoutRecipients,
			PopularityLookback: fPopularityLookback,
			Global:             global})
	version, _ := build.MainVersion()
	ln := &common.LeftNav{
		Cdc:     kReadOnlyCatDetailCache,
		Clock:   kClock,
		BuildId: build.BuildId(version),
	}
	http.Handle(
		"/fin/", &authHandler{mux})
	mux.Handle(
		"/fin/list",
		&list.Handler{
			Store:    kReadOnlyStore,
			Cdc:      kReadOnlyCatDetailCache,
			PageSize: kPageSize,
			Links:    fLinks,
			LN:       ln,
			Global:   global})
	mux.Handle(
		"/fin/recurringlist",
		&recurringlist.Handler{
			Doer:   kDoer,
			Cdc:    kReadOnlyCatDetailCache,
			Clock:  kClock,
			LN:     ln,
			Global: global})
	mux.Handle(
		"/fin/account",
		&account.Handler{
			Store:    kReadOnlyStore,
			Cdc:      kReadOnlyCatDetailCache,
			Doer:     kDoer,
			PageSize: kPageSize,
			Links:    fLinks,
			LN:       ln,
			Global:   global})
	mux.Handle(
		"/fin/single",
		&single.Handler{Doer: kDoer, Clock: kClock, Global: global, LN: ln})
	mux.Handle(
		"/fin/recurringsingle",
		&recurringsingle.Handler{
			Doer: kDoer, Clock: kClock, Global: global, LN: ln})
	mux.Handle("/fin/catedit", &catedit.Handler{LN: ln, Global: global})
	mux.Handle("/fin/logout", &logout.Handler{})
	// For now, the chpasswd handler gets full access to store
	mux.Handle(
		"/fin/chpasswd",
		&chpasswd.Handler{
			Store:  kStore,
			Doer:   kDoer,
			LN:     ln,
			Global: global})
	mux.Handle(
		"/fin/report",
		&report.Handler{
			Cdc:    kReadOnlyCatDetailCache,
			Store:  kReadOnlyStore,
			LN:     ln,
			Global: global,
			NoWifi: fNoWifi})
	mux.Handle(
		"/fin/trends",
		&trends.Handler{
			Store:  kReadOnlyStore,
			Cdc:    kReadOnlyCatDetailCache,
			LN:     ln,
			Global: global,
			NoWifi: fNoWifi})
	mux.Handle(
		"/fin/totals",
		&totals.Handler{Store: kReadOnlyStore, LN: ln, Global: global})
	mux.Handle(
		"/fin/export",
		&export.Handler{
			Store:  kReadOnlyStore,
			Cdc:    kReadOnlyCatDetailCache,
			Clock:  kClock,
			LN:     ln,
			Global: global})
	mux.Handle(
		"/fin/envelopes",
		&envelopes.Handler{
			Doer:   kDoer,
			LN:     ln,
			Global: global})
	mux.Handle(
		"/fin/addenvelope",
		&addenvelope.Handler{
			LN:     ln,
			Global: global})
	mux.Handle(
		"/fin/unreconciled",
		&unreconciled.Handler{
			Doer:     kDoer,
			PageSize: kPageSize,
			LN:       ln,
			Global:   global})
	mux.Handle(
		"/fin/unreviewed",
		&unreviewed.Handler{
			Doer:     kDoer,
			PageSize: kPageSize,
			LN:       ln,
			Global:   global})
	mux.Handle(
		"/fin/upload",
		&upload.Handler{Doer: kDoer, LN: ln, Global: global})
	mux.Handle(
		"/fin/acname",
		&ac.Handler{
			Store: kReadOnlyStore,
			Field: func(e fin.Entry) string { return e.Name }})
	mux.Handle(
		"/fin/acdesc",
		&ac.Handler{
			Store: kReadOnlyStore,
			Field: func(e fin.Entry) string { return e.Desc }})

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
			redirectString = "/fin/list"
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
		http_util.Redirect(w, r, "/fin/list")
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
	flag.IntVar(
		&fPopularityLookback,
		"popularity_lookback",
		200,
		"Number of entries to look back to find most popular categories")
	flag.BoolVar(&fNoWifi, "nowifi", false, "Run in nowifi mode")
}

func setupDb(filepath string) {
	rawdb, err := sql.Open("sqlite3", filepath)
	if err != nil {
		panic(err.Error())
	}
	dbase := sqlite3_db.New(rawdb)
	qfxdata := qfxsqlite.New(dbase)
	kDoer = sqlite3_db.NewDoer(dbase)
	kCatDetailCache = csqlite.New(dbase)
	kStore = for_sqlite.New(dbase)
	qfxLoader := qfx.QFXLoader{Store: qfxdata}
	csvLoader := csv.CsvLoader{Store: qfxdata}
	kUploaders = map[string]autoimport.Loader{
		".qfx": qfxLoader,
		".ofx": qfxLoader,
		".csv": csvLoader}
	kReadOnlyCatDetailCache = csqlite.ReadOnlyWrapper(kCatDetailCache)
	kReadOnlyStore = for_sqlite.ReadOnlyWrapper(kStore)
	readOnlyQFXLoader := qfx.QFXLoader{Store: qfxdb.ReadOnlyWrapper(qfxdata)}
	readOnlyCsvLoader := csv.CsvLoader{Store: qfxdb.ReadOnlyWrapper(qfxdata)}
	kReadOnlyUploaders = map[string]autoimport.Loader{
		".qfx": readOnlyQFXLoader,
		".ofx": readOnlyQFXLoader,
		".csv": readOnlyCsvLoader}
}

func setupStores(session *common.UserSession) bool {
	switch session.User.Permission {
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
	Email    string   `yaml:"email"`
	Password string   `yaml:"password"`
	To       []string `yaml:"to"`
	Failures int      `yaml:"failures"`
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
	if result.Failures <= 0 {
		return nil, errors.New("failures field must be positive")
	}
	if result.Email == "" && result.Password == "" && len(result.To) == 0 {
		// lockout without email notification
		return &result, nil
	}
	if result.Email == "" || result.Password == "" || len(result.To) == 0 {
		return nil, errors.New(
			"email, password, and to fields required")
	}
	return &result, nil
}

func setupGmail(configPath string) {
	gmailConfig, err := readGmailConfig(configPath)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}
	kLockout = lockout.New(gmailConfig.Failures)
	if gmailConfig.Email != "" {
		kMailer = mailer.New(gmailConfig.Email, gmailConfig.Password)
		kLockoutRecipients = gmailConfig.To
	}
}
