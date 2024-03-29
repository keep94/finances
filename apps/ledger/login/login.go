package login

import (
	"fmt"
	"github.com/keep94/finances/apps/ledger/common"
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/consumers"
	"github.com/keep94/finances/fin/findb"
	"github.com/keep94/sessions"
	"github.com/keep94/toolbox/db"
	"github.com/keep94/toolbox/http_util"
	"github.com/keep94/toolbox/lockout"
	"github.com/keep94/toolbox/mailer"
	"html/template"
	"net/http"
	"time"
)

const (
	kBadLoginMsg = "Login incorrect."
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
<h2>Login</h2>
{{if .Message}}
  <span class="error">{{.Message}}</span>
{{end}}
<form method="post">
  <table>
    <tr>
      <td>Name: </td>
      <td><input type="text" name="name"></td>
    </tr>
    <tr>
      <td>Password: </td>
      <td><input type="password" name="password"></td>
    </tr>
  </table>
  <br>
  <input type="submit" value="login">
</form>
</body>
</html>`
)

var (
	kTemplate *template.Template
)

type Sender interface {
	Send(email mailer.Email)
}

type Store interface {
	findb.UpdateUserByNameRunner
	findb.EntriesRunner
}

type Handler struct {
	Doer               db.Doer
	SessionStore       sessions.Store
	Store              Store
	LO                 *lockout.Lockout
	Mailer             Sender
	Recipients         []string
	PopularityLookback int
	Global             *common.Global
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		h.writeTemplate(w, "")
	} else {
		r.ParseForm()
		userName := r.Form.Get("name")
		password := r.Form.Get("password")
		if h.LO.Locked(userName) {
			h.writeTemplate(w, kBadLoginMsg)
			return
		}
		var user fin.User
		err := h.Doer.Do(func(t db.Transaction) error {
			return findb.LoginUser(t, h.Store, userName, password, time.Now(), &user)
		})
		if err == findb.WrongPassword {
			h.writeTemplate(w, kBadLoginMsg)
			if h.LO.Failure(userName) {
				h.sendLockoutEmail(userName)
			}
			return
		}
		if err == findb.NoSuchId {
			h.writeTemplate(w, kBadLoginMsg)
			return
		}
		if err != nil {
			http_util.ReportError(w, "Database error", err)
			return
		}
		gs, err := common.NewGorillaSession(h.SessionStore, r)
		if err != nil {
			http_util.ReportError(w, "Error retrieving session", err)
			return
		}
		h.LO.Success(userName)
		session := common.CreateUserSession(gs)
		// Just in case another user is already logged in
		session.ClearAll()
		session.SetUserId(user.Id)
		if !user.LastLogin.IsZero() {
			session.SetLastLogin(user.LastLogin)
		}
		if h.PopularityLookback > 0 {
			builder := consumers.NewCatPopularityBuilder(h.PopularityLookback)
			h.Store.Entries(nil, nil, builder)
			session.SetCatPopularity(builder.Build())
		}
		session.ID = "" // For added security, force a new session ID
		session.Save(r, w)
		prev := r.Form.Get("prev")
		if prev == "" {
			prev = "/fin/list"
		}
		http_util.Redirect(w, r, prev)
	}
}

func (h *Handler) sendLockoutEmail(userName string) {
	if h.Mailer == nil {
		return
	}
	subject := fmt.Sprintf("Account %s locked", userName)
	body := fmt.Sprintf(
		"Account %s locked from too many failed login attempts", userName)
	email := mailer.Email{
		To:      h.Recipients,
		Subject: subject,
		Body:    body}
	h.Mailer.Send(email)
}

func (h *Handler) writeTemplate(w http.ResponseWriter, message string) {
	http_util.WriteTemplate(w, kTemplate, &view{
		Message: message,
		Global:  h.Global})
}

type view struct {
	Message string
	Global  *common.Global
}

func init() {
	kTemplate = common.NewTemplate("login", kTemplateSpec)
}
