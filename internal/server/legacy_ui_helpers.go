package server

import (
	"html"
	"io/fs"
	"net/http"
	"strings"
)

func isHX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func setLangCookie(w http.ResponseWriter, lang string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "lang",
		Value:    lang,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func renderNav(r *http.Request) string {
	l := lang(r)
	return `<nav><ul>` +
		`<li><a href="/org/nodes" hx-get="/org/nodes" hx-target="#content" hx-push-url="true">` + tr(l, "nav_orgunit") + `</a></li>` +
		`<li><a href="/org/snapshot" hx-get="/org/snapshot" hx-target="#content" hx-push-url="true">` + tr(l, "nav_orgunit_snapshot") + `</a></li>` +
		`<li><a href="/org/setid" hx-get="/org/setid" hx-target="#content" hx-push-url="true">` + tr(l, "nav_setid") + `</a></li>` +
		`<li><a href="/org/job-catalog" hx-get="/org/job-catalog" hx-target="#content" hx-push-url="true">` + tr(l, "nav_jobcatalog") + `</a></li>` +
		`<li><a href="/org/positions" hx-get="/org/positions" hx-target="#content" hx-push-url="true">` + tr(l, "nav_staffing") + `</a></li>` +
		`<li><a href="/person/persons" hx-get="/person/persons" hx-target="#content" hx-push-url="true">` + tr(l, "nav_person") + `</a></li>` +
		`</ul></nav>`
}

func renderTopbar(r *http.Request) string {
	var b strings.Builder
	b.WriteString(`<div>`)
	b.WriteString(`<a href="/lang/en">EN</a> | <a href="/lang/zh">中文</a>`)
	b.WriteString(`</div>`)

	return b.String()
}

func lang(r *http.Request) string {
	c, err := r.Cookie("lang")
	if err != nil {
		return "en"
	}
	if c.Value == "zh" {
		return "zh"
	}
	return "en"
}

func tr(lang string, key string) string {
	if lang == "zh" {
		switch key {
		case "nav_orgunit":
			return "组织架构"
		case "nav_orgunit_snapshot":
			return "组织架构快照"
		case "nav_setid":
			return "SetID 管理"
		case "nav_jobcatalog":
			return "职位分类"
		case "nav_staffing":
			return "用工任职"
		case "nav_person":
			return "人员"
		case "as_of":
			return "有效日期"
		case "shared_readonly":
			return "共享/只读"
		case "tenant_owned":
			return "租户"
		}
	}

	switch key {
	case "nav_orgunit":
		return "OrgUnit"
	case "nav_orgunit_snapshot":
		return "OrgUnit Snapshot"
	case "nav_setid":
		return "SetID Governance"
	case "nav_jobcatalog":
		return "Job Catalog"
	case "nav_staffing":
		return "Staffing"
	case "nav_person":
		return "Person"
	case "as_of":
		return "As-of"
	case "shared_readonly":
		return "Shared/Read-only (共享/只读)"
	case "tenant_owned":
		return "Tenant"
	default:
		return ""
	}
}

func renderLoginForm(errMsg string) string {
	var b strings.Builder
	b.WriteString(`<h1>Login</h1>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00020">` + html.EscapeString(errMsg) + `</p>`)
	}
	b.WriteString(`<form method="POST" action="/login">`)
	b.WriteString(`<label>Email <input type="email" name="email" autocomplete="username" required></label><br>`)
	b.WriteString(`<label>Password <input type="password" name="password" autocomplete="current-password" required></label><br>`)
	b.WriteString(`<button type="submit">Login</button>`)
	b.WriteString(`</form>`)
	return b.String()
}

func writeShell(w http.ResponseWriter, r *http.Request, bodyHTML string) {
	writeShellWithStatus(w, r, http.StatusOK, bodyHTML)
}

func writeShellWithStatus(w http.ResponseWriter, r *http.Request, status int, bodyHTML string) {
	writeShellWithStatusFromAssets(w, r, status, bodyHTML, embeddedAssets)
}

func writeShellWithStatusFromAssets(w http.ResponseWriter, r *http.Request, status int, bodyHTML string, assets fs.FS) {
	_ = assets // DEV-PLAN-103: Astro shell removed; keep signature to avoid wide churn.

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(renderMinimalShell(bodyHTML)))
}

func writeContent(w http.ResponseWriter, _ *http.Request, bodyHTML string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(bodyHTML))
}

func writeContentWithStatus(w http.ResponseWriter, _ *http.Request, status int, bodyHTML string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(bodyHTML))
}

func writePage(w http.ResponseWriter, r *http.Request, bodyHTML string) {
	if isHX(r) {
		writeContent(w, r, bodyHTML)
		return
	}
	writeShell(w, r, bodyHTML)
}

func writePageWithStatus(w http.ResponseWriter, r *http.Request, status int, bodyHTML string) {
	if isHX(r) {
		writeContentWithStatus(w, r, status, bodyHTML)
		return
	}
	writeShellWithStatus(w, r, status, bodyHTML)
}

func renderMinimalShell(bodyHTML string) string {
	var b strings.Builder
	b.WriteString("<!doctype html><html><head>")
	b.WriteString(`<meta charset="utf-8">`)
	b.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1">`)
	b.WriteString("<title>Bugs &amp; Blossoms</title>")
	b.WriteString("</head><body>")
	b.WriteString(`<main id="content">`)
	b.WriteString(bodyHTML)
	b.WriteString("</main></body></html>")
	return b.String()
}

func redirectBack(w http.ResponseWriter, r *http.Request) {
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/app"
	}
	http.Redirect(w, r, ref, http.StatusFound)
}
