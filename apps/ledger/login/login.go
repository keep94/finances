package login

import (
  "github.com/gorilla/sessions"
  "github.com/keep94/appcommon/db"
  "github.com/keep94/appcommon/http_util"
  "github.com/keep94/finance/apps/ledger/common"
  "github.com/keep94/finance/fin"
  "github.com/keep94/finance/fin/findb"
  "html/template"
  "net/http"
  "time"
)

var (
  kTemplateSpec = `
<html>
<head>
  <link rel="stylesheet" type="text/css" href="/static/theme.css" />
</head>
<body>
<h2>Login</h2>
{{if .}}
  <span class="error">{{.}}</span>
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

type Handler struct {
  Doer db.Doer
  SessionStore sessions.Store
  Store findb.UpdateUserByNameRunner
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  if r.Method == "GET" {
    http_util.WriteTemplate(w, kTemplate, nil)
  } else {
    r.ParseForm()
    userName := r.Form.Get("name")
    password := r.Form.Get("password")
    var user fin.User
    err := h.Doer.Do(func(t db.Transaction) error {
      return findb.LoginUser(t, h.Store, userName, password, time.Now(), &user)
    })
    if err == findb.NoSuchId {
      http_util.WriteTemplate(w, kTemplate, "Login incorrect.")
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
    session := common.CreateUserSession(gs)
    // Just in case another user is already logged in
    session.ClearAll()
    session.SetUserId(user.Id)
    if !user.LastLogin.IsZero() {
      session.SetLastLogin(user.LastLogin)
    }
    session.ID = ""  // For added security, force a new session ID
    session.Save(r, w)
    http_util.Redirect(w, r, r.Form.Get("prev"))
  }
}

func init() {
  kTemplate = common.NewTemplate("login", kTemplateSpec)
}
