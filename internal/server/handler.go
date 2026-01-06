package server

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

//go:embed assets/*
var embeddedAssets embed.FS

func NewHandler() (http.Handler, error) {
	return NewHandlerWithOptions(HandlerOptions{})
}

type HandlerOptions struct {
	OrgUnitStore    OrgUnitStore
	OrgUnitSnapshot OrgUnitSnapshotStore
}

func NewHandlerWithOptions(opts HandlerOptions) (http.Handler, error) {
	tenants, err := loadTenants()
	if err != nil {
		return nil, err
	}

	allowlistPath := os.Getenv("ALLOWLIST_PATH")
	if allowlistPath == "" {
		p, err := defaultAllowlistPath()
		if err != nil {
			return nil, err
		}
		allowlistPath = p
	}

	a, err := routing.LoadAllowlist(allowlistPath)
	if err != nil {
		return nil, err
	}

	classifier, err := routing.NewClassifier(a, "server")
	if err != nil {
		return nil, err
	}

	orgStore := opts.OrgUnitStore
	orgSnapshotStore := opts.OrgUnitSnapshot
	if orgStore == nil {
		dsn := dbDSNFromEnv()
		pool, err := pgxpool.New(context.Background(), dsn)
		if err != nil {
			return nil, err
		}
		orgStore = newOrgUnitPGStore(pool)
	}

	if orgSnapshotStore == nil {
		if pgStore, ok := orgStore.(*orgUnitPGStore); ok {
			orgSnapshotStore = newOrgUnitSnapshotPGStore(pgStore.pool)
		}
	}

	router := routing.NewRouter(classifier)
	guarded := withTenantAndSession(tenants, router)

	router.Handle(routing.RouteClassUI, http.MethodGet, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app", http.StatusFound)
	}))

	router.Handle(routing.RouteClassOps, http.MethodGet, "/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	}))
	router.Handle(routing.RouteClassOps, http.MethodGet, "/healthz", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	}))

	router.Handle(routing.RouteClassAuthn, http.MethodGet, "/login", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeShell(w, r, `<h1>Login</h1>`+
			`<form method="POST" action="/login">`+
			`<button type="submit">Login</button>`+
			`</form>`)
	}))
	router.Handle(routing.RouteClassAuthn, http.MethodPost, "/login", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSessionCookie(w)
		http.Redirect(w, r, "/app", http.StatusFound)
	}))
	router.Handle(routing.RouteClassAuthn, http.MethodPost, "/logout", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clearSessionCookie(w)
		http.Redirect(w, r, "/login", http.StatusFound)
	}))

	router.Handle(routing.RouteClassUI, http.MethodGet, "/lang/en", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setLangCookie(w, "en")
		redirectBack(w, r)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/lang/zh", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setLangCookie(w, "zh")
		redirectBack(w, r)
	}))

	router.Handle(routing.RouteClassUI, http.MethodGet, "/app", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeShell(w, r, `<div id="content" hx-get="/app/home" hx-trigger="load"></div>`)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/app/home", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := lang(r)
		writeContent(w, r, `<h1>Home</h1>`+
			`<p>Pick a module:</p>`+
			`<ul>`+
			`<li><a href="/org/nodes" hx-get="/org/nodes" hx-target="#content" hx-push-url="true">`+tr(l, "nav_orgunit")+`</a></li>`+
			`<li><a href="/org/snapshot" hx-get="/org/snapshot" hx-target="#content" hx-push-url="true">`+tr(l, "nav_orgunit_snapshot")+`</a></li>`+
			`<li><a href="/org/job-catalog" hx-get="/org/job-catalog" hx-target="#content" hx-push-url="true">`+tr(l, "nav_jobcatalog")+`</a></li>`+
			`<li><a href="/org/positions" hx-get="/org/positions" hx-target="#content" hx-push-url="true">`+tr(l, "nav_staffing")+`</a></li>`+
			`<li><a href="/person/persons" hx-get="/person/persons" hx-target="#content" hx-push-url="true">`+tr(l, "nav_person")+`</a></li>`+
			`</ul>`)
	}))

	router.Handle(routing.RouteClassUI, http.MethodGet, "/ui/nav", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeContent(w, r, renderNav(r))
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/ui/topbar", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeContent(w, r, renderTopbar(r))
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/ui/flash", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeContent(w, r, "")
	}))

	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/nodes", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgNodes(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/nodes", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgNodes(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/snapshot", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgSnapshot(w, r, orgSnapshotStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/snapshot", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgSnapshot(w, r, orgSnapshotStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/job-catalog", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeShell(w, r, "<h1>Job Catalog /org/job-catalog</h1><p>(placeholder)</p>")
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/positions", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeShell(w, r, "<h1>Staffing /org/positions</h1><p>(placeholder)</p>")
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/assignments", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeShell(w, r, "<h1>Staffing /org/assignments</h1><p>(placeholder)</p>")
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/person/persons", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeShell(w, r, "<h1>Person /person/persons</h1><p>(placeholder)</p>")
	}))

	assetsSub, _ := fs.Sub(embeddedAssets, "assets")

	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsSub))))
	mux.Handle("/", guarded)

	return mux, nil
}

func MustNewHandler() http.Handler {
	h, err := NewHandler()
	if err != nil {
		panic(errors.New("server: failed to build handler: " + err.Error()))
	}
	return h
}

func defaultAllowlistPath() (string, error) {
	path := "config/routing/allowlist.yaml"
	for range 8 {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		path = filepath.Join("..", path)
	}
	return "", errors.New("server: allowlist not found")
}

func redirectBack(w http.ResponseWriter, r *http.Request) {
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/app"
	}
	http.Redirect(w, r, ref, http.StatusFound)
}

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

func setSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "ok",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func withTenantAndSession(tenants map[string]Tenant, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/health" || path == "/healthz" || path == "/assets" || pathHasPrefixSegment(path, "/assets") {
			next.ServeHTTP(w, r)
			return
		}

		tenantDomain := hostWithoutPort(r.Host)
		if tenantDomain == "" {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "tenant_not_found", "tenant not found")
			return
		}
		t, ok := tenants[tenantDomain]
		if !ok {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "tenant_not_found", "tenant not found")
			return
		}
		r = r.WithContext(withTenant(r.Context(), t))

		if path == "/login" || pathHasPrefixSegment(path, "/lang") {
			next.ServeHTTP(w, r)
			return
		}

		if !hasSession(r) {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func hasSession(r *http.Request) bool {
	c, err := r.Cookie("session")
	if err != nil {
		return false
	}
	return c.Value == "ok"
}

func pathHasPrefixSegment(path, prefix string) bool {
	if path == prefix {
		return true
	}
	return len(path) > len(prefix) && path[:len(prefix)+1] == prefix+"/"
}

func renderNav(r *http.Request) string {
	l := lang(r)
	return `<nav><ul>` +
		`<li><a href="/org/nodes" hx-get="/org/nodes" hx-target="#content" hx-push-url="true">` + tr(l, "nav_orgunit") + `</a></li>` +
		`<li><a href="/org/snapshot" hx-get="/org/snapshot" hx-target="#content" hx-push-url="true">` + tr(l, "nav_orgunit_snapshot") + `</a></li>` +
		`<li><a href="/org/job-catalog" hx-get="/org/job-catalog" hx-target="#content" hx-push-url="true">` + tr(l, "nav_jobcatalog") + `</a></li>` +
		`<li><a href="/org/positions" hx-get="/org/positions" hx-target="#content" hx-push-url="true">` + tr(l, "nav_staffing") + `</a></li>` +
		`<li><a href="/person/persons" hx-get="/person/persons" hx-target="#content" hx-push-url="true">` + tr(l, "nav_person") + `</a></li>` +
		`</ul></nav>`
}

func renderTopbar(r *http.Request) string {
	l := lang(r)
	return `<div>` +
		`<a href="/lang/en">EN</a> | <a href="/lang/zh">中文</a>` +
		`<span style="margin-left:12px">` + tr(l, "as_of") + `</span>` +
		`<input type="date" name="as_of" />` +
		`</div>`
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
		case "nav_jobcatalog":
			return "职位分类"
		case "nav_staffing":
			return "用工任职"
		case "nav_person":
			return "人员"
		case "as_of":
			return "有效日期"
		}
	}

	switch key {
	case "nav_orgunit":
		return "OrgUnit"
	case "nav_orgunit_snapshot":
		return "OrgUnit Snapshot"
	case "nav_jobcatalog":
		return "Job Catalog"
	case "nav_staffing":
		return "Staffing"
	case "nav_person":
		return "Person"
	case "as_of":
		return "As-of"
	default:
		return ""
	}
}

func writeShell(w http.ResponseWriter, r *http.Request, bodyHTML string) {
	l := lang(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!doctype html><html><head>`))
	_, _ = w.Write([]byte(`<meta charset="utf-8">`))
	_, _ = w.Write([]byte(`<title>` + tr(l, "nav_orgunit") + `</title>`))
	_, _ = w.Write([]byte(`<link rel="stylesheet" href="/assets/app.css">`))
	_, _ = w.Write([]byte(`<script src="/assets/js/lib/htmx.min.js"></script>`))
	_, _ = w.Write([]byte(`<script defer src="/assets/js/lib/alpine.min.js"></script>`))
	_, _ = w.Write([]byte(`</head><body>`))
	if hasSession(r) {
		_, _ = w.Write([]byte(`<aside id="nav" hx-get="/ui/nav" hx-trigger="load"></aside>`))
		_, _ = w.Write([]byte(`<header id="topbar" hx-get="/ui/topbar" hx-trigger="load"></header>`))
		_, _ = w.Write([]byte(`<div id="flash" hx-get="/ui/flash" hx-trigger="load"></div>`))
	}
	_, _ = w.Write([]byte(`<main id="content">` + bodyHTML + `</main>`))
	_, _ = w.Write([]byte(`</body></html>`))
	_ = l
}

func writeContent(w http.ResponseWriter, _ *http.Request, bodyHTML string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(bodyHTML))
}

func writePage(w http.ResponseWriter, r *http.Request, bodyHTML string) {
	if isHX(r) {
		writeContent(w, r, bodyHTML)
		return
	}
	writeShell(w, r, bodyHTML)
}
