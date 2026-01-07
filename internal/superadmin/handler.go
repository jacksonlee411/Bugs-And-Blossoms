package superadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

func NewHandler() (http.Handler, error) {
	return NewHandlerWithOptions(HandlerOptions{})
}

type HandlerOptions struct {
	Pool pgBeginner
}

func NewHandlerWithOptions(opts HandlerOptions) (http.Handler, error) {
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

	classifier, err := routing.NewClassifier(a, "superadmin")
	if err != nil {
		return nil, err
	}
	router := routing.NewRouter(classifier)

	pool := opts.Pool
	if pool == nil {
		dsn, err := dbDSNFromEnv()
		if err != nil {
			return nil, err
		}
		p, err := pgxpool.New(context.Background(), dsn)
		if err != nil {
			return nil, err
		}
		pool = p
	}

	authorizer, err := loadAuthorizer()
	if err != nil {
		return nil, err
	}

	guarded := withBasicAuth(withAuthz(classifier, authorizer, router))

	router.Handle(routing.RouteClassUI, http.MethodGet, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/superadmin/tenants", http.StatusFound)
	}))

	router.Handle(routing.RouteClassOps, http.MethodGet, "/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	}))
	router.Handle(routing.RouteClassOps, http.MethodGet, "/healthz", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	}))

	router.Handle(routing.RouteClassUI, http.MethodGet, "/superadmin/tenants", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleTenantsIndex(w, r, pool)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/superadmin/tenants", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleTenantsCreate(w, r, pool)
	}))

	router.Handle(routing.RouteClassUI, http.MethodPost, "/superadmin/tenants/{tenant_id}/enable", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleTenantToggle(w, r, pool, true)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/superadmin/tenants/{tenant_id}/disable", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleTenantToggle(w, r, pool, false)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/superadmin/tenants/{tenant_id}/domains", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleTenantBindDomain(w, r, pool)
	}))

	mux := http.NewServeMux()
	mux.Handle("/", guarded)
	return mux, nil
}

func MustNewHandler() http.Handler {
	h, err := NewHandler()
	if err != nil {
		panic(errors.New("superadmin: failed to build handler: " + err.Error()))
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
	return "", errors.New("superadmin: allowlist not found")
}

type authorizer interface {
	Authorize(subject string, domain string, object string, action string) (allowed bool, enforced bool, err error)
}

func withAuthz(classifier *routing.Classifier, a authorizer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		rc := routing.RouteClassUI
		if classifier != nil {
			rc = classifier.Classify(path)
		}

		switch path {
		case "/health", "/healthz":
			next.ServeHTTP(w, r)
			return
		default:
		}

		roleSlug := authz.RoleSuperadmin
		subject := authz.SubjectFromRoleSlug(roleSlug)
		domain := authz.DomainGlobal

		object, action, shouldCheck := authzRequirementForRoute(r.Method, path)
		if !shouldCheck {
			next.ServeHTTP(w, r)
			return
		}

		allowed, enforced, err := a.Authorize(subject, domain, object, action)
		if err != nil {
			routing.WriteError(w, r, rc, http.StatusInternalServerError, "authz_error", "authz error")
			return
		}
		if enforced && !allowed {
			routing.WriteError(w, r, rc, http.StatusForbidden, "forbidden", "forbidden")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func authzRequirementForRoute(method string, path string) (object string, action string, ok bool) {
	switch path {
	case "/superadmin/tenants":
		if method == http.MethodGet {
			return authz.ObjectSuperadminTenants, authz.ActionRead, true
		}
		if method == http.MethodPost {
			return authz.ObjectSuperadminTenants, authz.ActionAdmin, true
		}
		return "", "", false
	default:
		if strings.HasPrefix(path, "/superadmin/tenants/") && method == http.MethodPost {
			return authz.ObjectSuperadminTenants, authz.ActionAdmin, true
		}
		return "", "", false
	}
}

type pgBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func writeHTML(w http.ResponseWriter, title string, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "<!doctype html><html><head><title>%s</title></head><body>%s</body></html>", html.EscapeString(title), body)
}

func requestID(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Request-Id")); v != "" {
		return v
	}
	return fmt.Sprintf("sa-%d", time.Now().UnixNano())
}

func superadminWritesEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("SUPERADMIN_WRITE_MODE")))
	if v == "" {
		return true
	}
	return v == "enabled"
}

type tenantRow struct {
	ID          string
	Name        string
	IsActive    bool
	PrimaryHost string
	OtherHosts  []string
}

func handleTenantsIndex(w http.ResponseWriter, r *http.Request, pool pgBeginner) {
	ctx := r.Context()
	rows, err := pool.Query(ctx, `
SELECT id::text, name, is_active
FROM iam.tenants
ORDER BY created_at ASC, id ASC
`)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
		return
	}
	defer rows.Close()

	tenants := make([]tenantRow, 0, 8)
	byID := make(map[string]*tenantRow)
	for rows.Next() {
		var tr tenantRow
		if err := rows.Scan(&tr.ID, &tr.Name, &tr.IsActive); err != nil {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
			return
		}
		tenants = append(tenants, tr)
		byID[tr.ID] = &tenants[len(tenants)-1]
	}
	if err := rows.Err(); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
		return
	}

	domainRows, err := pool.Query(ctx, `
SELECT tenant_id::text, hostname, is_primary
FROM iam.tenant_domains
ORDER BY is_primary DESC, hostname ASC
`)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
		return
	}
	defer domainRows.Close()
	for domainRows.Next() {
		var tenantID string
		var hostname string
		var isPrimary bool
		if err := domainRows.Scan(&tenantID, &hostname, &isPrimary); err != nil {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
			return
		}
		tr, ok := byID[tenantID]
		if !ok {
			continue
		}
		if isPrimary && tr.PrimaryHost == "" {
			tr.PrimaryHost = hostname
		} else {
			tr.OtherHosts = append(tr.OtherHosts, hostname)
		}
	}
	if err := domainRows.Err(); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
		return
	}

	var b strings.Builder
	b.WriteString("<h1>SuperAdmin / Tenants</h1>")
	b.WriteString("<h2>Create tenant</h2>")
	b.WriteString(`<form method="POST" action="/superadmin/tenants">`)
	b.WriteString(`<div><label>Name <input name="name" /></label></div>`)
	b.WriteString(`<div><label>Primary Hostname <input name="hostname" placeholder="example.local" /></label></div>`)
	b.WriteString(`<div><button type="submit">Create</button></div>`)
	b.WriteString(`</form>`)

	b.WriteString("<h2>Existing tenants</h2>")
	if len(tenants) == 0 {
		b.WriteString("<p>(none)</p>")
		writeHTML(w, "Tenants", b.String())
		return
	}

	b.WriteString(`<table border="1" cellpadding="6" cellspacing="0">`)
	b.WriteString("<thead><tr><th>ID</th><th>Name</th><th>Active</th><th>Domains</th><th>Actions</th></tr></thead><tbody>")
	for _, t := range tenants {
		b.WriteString("<tr>")
		b.WriteString("<td><code>" + html.EscapeString(t.ID) + "</code></td>")
		b.WriteString("<td>" + html.EscapeString(t.Name) + "</td>")
		if t.IsActive {
			b.WriteString("<td>yes</td>")
		} else {
			b.WriteString("<td>no</td>")
		}
		b.WriteString("<td>")
		if t.PrimaryHost != "" {
			b.WriteString("<div><b>primary</b>: " + html.EscapeString(t.PrimaryHost) + "</div>")
		}
		for _, h := range t.OtherHosts {
			b.WriteString("<div>" + html.EscapeString(h) + "</div>")
		}
		b.WriteString("</td>")
		b.WriteString("<td>")
		if t.IsActive {
			b.WriteString(fmt.Sprintf(`<form method="POST" action="/superadmin/tenants/%s/disable"><button type="submit">Disable</button></form>`, html.EscapeString(t.ID)))
		} else {
			b.WriteString(fmt.Sprintf(`<form method="POST" action="/superadmin/tenants/%s/enable"><button type="submit">Enable</button></form>`, html.EscapeString(t.ID)))
		}
		b.WriteString(fmt.Sprintf(`<form method="POST" action="/superadmin/tenants/%s/domains">`, html.EscapeString(t.ID)))
		b.WriteString(`<input name="hostname" placeholder="add hostname" /> <button type="submit">Bind Domain</button>`)
		b.WriteString(`</form>`)
		b.WriteString("</td>")
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>")

	writeHTML(w, "Tenants", b.String())
}

func handleTenantsCreate(w http.ResponseWriter, r *http.Request, pool pgBeginner) {
	if !superadminWritesEnabled() {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusForbidden, "write_disabled", "write disabled")
		return
	}

	if err := r.ParseForm(); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "bad_request", "bad request")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	hostname := strings.ToLower(strings.TrimSpace(r.FormValue("hostname")))
	hostname = strings.TrimSpace(hostname)
	if name == "" || hostname == "" {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_input", "invalid input")
		return
	}
	if strings.Contains(hostname, ":") || strings.ContainsAny(hostname, " \t\r\n") {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_hostname", "invalid hostname")
		return
	}

	ctx := r.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var tenantID string
	if err := tx.QueryRow(ctx, `
INSERT INTO iam.tenants(name, is_active)
VALUES ($1, true)
RETURNING id::text
`, name).Scan(&tenantID); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
		return
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO iam.tenant_domains(tenant_id, hostname, is_primary)
VALUES ($1::uuid, $2, true)
`, tenantID, hostname); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
		return
	}

	actor, _ := actorFromContext(r.Context())
	payload, _ := json.Marshal(map[string]any{"name": name, "hostname": hostname})
	if err := insertAudit(ctx, tx, actor, "tenant.create", tenantID, payload, requestID(r)); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "audit_error", "audit error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
		return
	}

	http.Redirect(w, r, "/superadmin/tenants", http.StatusFound)
}

func handleTenantToggle(w http.ResponseWriter, r *http.Request, pool pgBeginner, enable bool) {
	if !superadminWritesEnabled() {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusForbidden, "write_disabled", "write disabled")
		return
	}

	tenantID, ok := tenantIDFromPath(r.URL.Path)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "bad_request", "bad request")
		return
	}

	ctx := r.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `
UPDATE iam.tenants
SET is_active = $2, updated_at = now()
WHERE id = $1::uuid
`, tenantID, enable); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
		return
	}

	actor, _ := actorFromContext(r.Context())
	action := "tenant.disable"
	if enable {
		action = "tenant.enable"
	}
	payload, _ := json.Marshal(map[string]any{"enable": enable})
	if err := insertAudit(ctx, tx, actor, action, tenantID, payload, requestID(r)); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "audit_error", "audit error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
		return
	}
	http.Redirect(w, r, "/superadmin/tenants", http.StatusFound)
}

func handleTenantBindDomain(w http.ResponseWriter, r *http.Request, pool pgBeginner) {
	if !superadminWritesEnabled() {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusForbidden, "write_disabled", "write disabled")
		return
	}

	tenantID, ok := tenantIDFromPath(r.URL.Path)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "bad_request", "bad request")
		return
	}
	if err := r.ParseForm(); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "bad_request", "bad request")
		return
	}
	hostname := strings.ToLower(strings.TrimSpace(r.FormValue("hostname")))
	hostname = strings.TrimSpace(hostname)
	if hostname == "" || strings.Contains(hostname, ":") || strings.ContainsAny(hostname, " \t\r\n") {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_hostname", "invalid hostname")
		return
	}

	ctx := r.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `
INSERT INTO iam.tenant_domains(tenant_id, hostname, is_primary)
VALUES ($1::uuid, $2, false)
`, tenantID, hostname); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
		return
	}

	actor, _ := actorFromContext(r.Context())
	payload, _ := json.Marshal(map[string]any{"hostname": hostname})
	if err := insertAudit(ctx, tx, actor, "tenant.domain.bind", tenantID, payload, requestID(r)); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "audit_error", "audit error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "db_error", "db error")
		return
	}
	http.Redirect(w, r, "/superadmin/tenants", http.StatusFound)
}

func tenantIDFromPath(path string) (string, bool) {
	// /superadmin/tenants/{tenant_id}/...
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 {
		return "", false
	}
	if parts[0] != "superadmin" || parts[1] != "tenants" {
		return "", false
	}
	return parts[2], true
}

func insertAudit(ctx context.Context, tx pgx.Tx, actor string, action string, tenantID string, payload []byte, reqID string) error {
	if actor == "" {
		actor = "unknown"
	}
	if payload == nil {
		payload = []byte(`{}`)
	}
	_, err := tx.Exec(ctx, `
INSERT INTO iam.superadmin_audit_logs(actor, action, target_tenant_id, payload, request_id)
VALUES ($1, $2, $3::uuid, $4::jsonb, $5)
`, actor, action, tenantID, payload, reqID)
	return err
}
