package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"
	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type staticIdentityProvider struct {
	ident authenticatedIdentity
	err   error
}

type fakePGBeginner struct{}

func (fakePGBeginner) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("boom")
}

func (s staticIdentityProvider) AuthenticatePassword(context.Context, Tenant, string, string) (authenticatedIdentity, error) {
	return s.ident, s.err
}

func localTenancyResolver() TenancyResolver {
	return newStaticTenancyResolver(map[string]Tenant{
		"localhost": {ID: "00000000-0000-0000-0000-000000000001", Domain: "localhost", Name: "Local Tenant"},
	})
}

func TestNewHandler_Health(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	h, err := NewHandler()
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status=%d", rec2.Code)
	}
}

func TestLogin_UsesDefaultKratosIdentityProviderWhenNil(t *testing.T) {
	type stubState struct {
		loginStatus int
		whoamiID    string
		tenantID    string
		email       string
	}

	st := &stubState{
		loginStatus: http.StatusOK,
		whoamiID:    "kid1",
		tenantID:    "00000000-0000-0000-0000-000000000001",
		email:       "tenant-admin@example.invalid",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/self-service/login/api", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method=%s", r.Method)
		}
		_, _ = w.Write([]byte(`{"id":"flow1"}`))
	})
	mux.HandleFunc("/self-service/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		if st.loginStatus/100 != 2 {
			w.WriteHeader(st.loginStatus)
			return
		}
		_, _ = w.Write([]byte(`{"session_token":"st1"}`))
	})
	mux.HandleFunc("/sessions/whoami", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method=%s", r.Method)
		}
		if r.Header.Get("X-Session-Token") != "st1" {
			t.Fatalf("missing session token header")
		}
		_, _ = w.Write([]byte(`{"identity":{"id":"` + st.whoamiID + `","traits":{"tenant_uuid":"` + st.tenantID + `","email":"` + st.email + `"}}}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("KRATOS_PUBLIC_URL", srv.URL)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=tenant-admin%40example.invalid&password=pw"))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAppHome_ServesWebMUIIndexWithoutAsOf(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{Email: "tenant-admin@example.invalid", KratosIdentityID: "kid1"}},
		OrgUnitStore:     newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=tenant-admin%40example.invalid&password=pw"))
	loginReq.Host = "localhost:8080"
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusFound {
		t.Fatalf("login status=%d", loginRec.Code)
	}

	sidCookie := loginRec.Result().Cookies()[0]
	if sidCookie == nil || sidCookie.Name != "sid" || sidCookie.Value == "" {
		t.Fatalf("unexpected sid cookie: %#v", sidCookie)
	}

	req := httptest.NewRequest(http.MethodGet, "/app/home", nil)
	req.Host = "localhost:8080"
	req.AddCookie(sidCookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, `<div id="root"></div>`) {
		t.Fatalf("unexpected body=%q", body)
	}
}

func TestServeWebMUIIndex_MissingAsset(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/app", nil)
	rec := httptest.NewRecorder()
	serveWebMUIIndex(rec, req, fstest.MapFS{})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestNewHandler_DefaultPath(t *testing.T) {
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	h, err := NewHandler()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestNewHandler_ClassifierError(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "allowlist.yaml")
	if err := os.WriteFile(p, []byte("version: 1\nentrypoints:\n  server:\n    routes: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("ALLOWLIST_PATH", p); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	_, err := NewHandler()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandler_DefaultAllowlistNotFound(t *testing.T) {
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	_, err = NewHandler()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandlerWithOptions_PGStoreDefaults(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    &orgUnitPGStore{pool: &fakeBeginner{}},
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestNewHandlerWithOptions_MissingTenancyResolver_Error(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)

	_, err = NewHandlerWithOptions(HandlerOptions{
		OrgUnitStore: newOrgUnitMemoryStore(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandlerWithOptions_OrgUnitWriteServiceDefault(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	_, err = NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    &orgUnitPGStore{pool: fakePGBeginner{}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUI_ShellAndPartials(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{
			KratosIdentityID: "00000000-0000-0000-0000-0000000000aa",
			Email:            "tenant-admin@example.invalid",
		}},
		OrgUnitStore: newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	reqAsset := httptest.NewRequest(http.MethodGet, "/assets/app.css", nil)
	recAsset := httptest.NewRecorder()
	h.ServeHTTP(recAsset, reqAsset)
	if recAsset.Code != http.StatusOK {
		t.Fatalf("asset status=%d", recAsset.Code)
	}

	reqNoTenant := httptest.NewRequest(http.MethodGet, "/login", nil)
	reqNoTenant.Host = ""
	recNoTenant := httptest.NewRecorder()
	h.ServeHTTP(recNoTenant, reqNoTenant)
	if recNoTenant.Code != http.StatusNotFound {
		t.Fatalf("no-tenant status=%d", recNoTenant.Code)
	}

	reqBadTenant := httptest.NewRequest(http.MethodGet, "/login", nil)
	reqBadTenant.Host = "nope:8080"
	recBadTenant := httptest.NewRecorder()
	h.ServeHTTP(recBadTenant, reqBadTenant)
	if recBadTenant.Code != http.StatusNotFound {
		t.Fatalf("bad-tenant status=%d", recBadTenant.Code)
	}

	reqLogin := httptest.NewRequest(http.MethodGet, "/login", nil)
	reqLogin.Host = "localhost:8080"
	recLogin := httptest.NewRecorder()
	h.ServeHTTP(recLogin, reqLogin)
	if recLogin.Code != http.StatusOK {
		t.Fatalf("login status=%d", recLogin.Code)
	}
	if body := recLogin.Body.String(); !strings.Contains(body, `<form method="POST" action="/login">`) {
		t.Fatalf("unexpected login body: %q", body)
	}
	if body := recLogin.Body.String(); strings.Contains(body, `hx-trigger="load"`) {
		t.Fatalf("expected hx-trigger removed in login body: %q", body)
	}

	reqAppNoSession := httptest.NewRequest(http.MethodGet, "/app", nil)
	reqAppNoSession.Host = "localhost:8080"
	recAppNoSession := httptest.NewRecorder()
	h.ServeHTTP(recAppNoSession, reqAppNoSession)
	if recAppNoSession.Code != http.StatusFound {
		t.Fatalf("app (no session) status=%d", recAppNoSession.Code)
	}

	reqLoginPost := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=tenant-admin%40example.invalid&password=pw"))
	reqLoginPost.Host = "localhost:8080"
	reqLoginPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recLoginPost := httptest.NewRecorder()
	h.ServeHTTP(recLoginPost, reqLoginPost)
	if recLoginPost.Code != http.StatusFound {
		t.Fatalf("login post status=%d", recLoginPost.Code)
	}
	var session *http.Cookie
	for _, c := range recLoginPost.Result().Cookies() {
		if c.Name == sidCookieName {
			session = c
			break
		}
	}
	if session == nil || session.Value == "" {
		t.Fatalf("missing %s cookie", sidCookieName)
	}

	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	reqRoot.Host = "localhost:8080"
	reqRoot.AddCookie(session)
	recRoot := httptest.NewRecorder()
	h.ServeHTTP(recRoot, reqRoot)
	if recRoot.Code != http.StatusFound {
		t.Fatalf("root status=%d", recRoot.Code)
	}

	protected := []string{
		"/app?as_of=2026-01-01",
		"/app/home?as_of=2026-01-01",
		"/ui/flash",
		"/ui/nav",
		"/ui/topbar",
		"/org/nodes?tree_as_of=2026-01-01",
		"/org/snapshot?as_of=2026-01-01",
		"/org/setid?as_of=2026-01-01",
		"/org/job-catalog?as_of=2026-01-01",
		"/org/positions?as_of=2026-01-01",
		"/org/assignments?as_of=2026-01-01",
		"/person/persons?as_of=2026-01-01",
	}
	for _, p := range protected {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		req.Host = "localhost:8080"
		req.AddCookie(session)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("path=%s status=%d", p, rec.Code)
		}
	}

	reqAppMissingAsOf := httptest.NewRequest(http.MethodGet, "/app", nil)
	reqAppMissingAsOf.Host = "localhost:8080"
	reqAppMissingAsOf.AddCookie(session)
	recAppMissingAsOf := httptest.NewRecorder()
	h.ServeHTTP(recAppMissingAsOf, reqAppMissingAsOf)
	if recAppMissingAsOf.Code != http.StatusOK {
		t.Fatalf("app (missing as_of) status=%d", recAppMissingAsOf.Code)
	}

	reqNavMissingAsOf := httptest.NewRequest(http.MethodGet, "/ui/nav", nil)
	reqNavMissingAsOf.Host = "localhost:8080"
	reqNavMissingAsOf.AddCookie(session)
	recNavMissingAsOf := httptest.NewRecorder()
	h.ServeHTTP(recNavMissingAsOf, reqNavMissingAsOf)
	if recNavMissingAsOf.Code != http.StatusOK {
		t.Fatalf("nav (missing as_of) status=%d", recNavMissingAsOf.Code)
	}

	reqTopbarMissingAsOf := httptest.NewRequest(http.MethodGet, "/ui/topbar", nil)
	reqTopbarMissingAsOf.Host = "localhost:8080"
	reqTopbarMissingAsOf.AddCookie(session)
	recTopbarMissingAsOf := httptest.NewRecorder()
	h.ServeHTTP(recTopbarMissingAsOf, reqTopbarMissingAsOf)
	if recTopbarMissingAsOf.Code != http.StatusOK {
		t.Fatalf("topbar (missing as_of) status=%d", recTopbarMissingAsOf.Code)
	}

	reqSetIDPost := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader("action=create_setid&setid=A0001&name=Default+A"))
	reqSetIDPost.Host = "localhost:8080"
	reqSetIDPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqSetIDPost.AddCookie(session)
	recSetIDPost := httptest.NewRecorder()
	h.ServeHTTP(recSetIDPost, reqSetIDPost)
	if recSetIDPost.Code != http.StatusSeeOther {
		t.Fatalf("setid post status=%d", recSetIDPost.Code)
	}

	reqJobCatalogPost := httptest.NewRequest(http.MethodPost, "/org/job-catalog?as_of=2026-01-01&package_code=DEFLT", strings.NewReader("action=create_job_family_group&effective_date=2026-01-01&package_code=DEFLT&job_family_group_code=JC1&job_family_group_name=Group1"))
	reqJobCatalogPost.Host = "localhost:8080"
	reqJobCatalogPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqJobCatalogPost.AddCookie(session)
	recJobCatalogPost := httptest.NewRecorder()
	h.ServeHTTP(recJobCatalogPost, reqJobCatalogPost)
	if recJobCatalogPost.Code != http.StatusSeeOther {
		t.Fatalf("jobcatalog post status=%d", recJobCatalogPost.Code)
	}

	reqOrgSnapshotPost := httptest.NewRequest(http.MethodPost, "/org/snapshot?as_of=2026-01-01", strings.NewReader("name=A"))
	reqOrgSnapshotPost.Host = "localhost:8080"
	reqOrgSnapshotPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqOrgSnapshotPost.AddCookie(session)
	recOrgSnapshotPost := httptest.NewRecorder()
	h.ServeHTTP(recOrgSnapshotPost, reqOrgSnapshotPost)
	if recOrgSnapshotPost.Code != http.StatusOK {
		t.Fatalf("org snapshot post status=%d", recOrgSnapshotPost.Code)
	}

	reqCreate := httptest.NewRequest(http.MethodPost, "/org/nodes?tree_as_of=2026-01-01", strings.NewReader("org_code=ORG-1&name=NodeA"))
	reqCreate.Host = "localhost:8080"
	reqCreate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqCreate.AddCookie(session)
	recCreate := httptest.NewRecorder()
	h.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusSeeOther {
		t.Fatalf("org create status=%d", recCreate.Code)
	}

	reqChildren := httptest.NewRequest(http.MethodGet, "/org/nodes/children?tree_as_of=2026-01-01&parent_id=10000000", nil)
	reqChildren.Host = "localhost:8080"
	reqChildren.AddCookie(session)
	recChildren := httptest.NewRecorder()
	h.ServeHTTP(recChildren, reqChildren)
	if recChildren.Code != http.StatusOK {
		t.Fatalf("org children status=%d", recChildren.Code)
	}

	reqDetails := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=2026-01-01&org_id=10000000", nil)
	reqDetails.Host = "localhost:8080"
	reqDetails.AddCookie(session)
	recDetails := httptest.NewRecorder()
	h.ServeHTTP(recDetails, reqDetails)
	if recDetails.Code != http.StatusOK {
		t.Fatalf("org details status=%d", recDetails.Code)
	}

	reqSearch := httptest.NewRequest(http.MethodGet, "/org/nodes/search?tree_as_of=2026-01-01&query=ORG-1", nil)
	reqSearch.Host = "localhost:8080"
	reqSearch.AddCookie(session)
	recSearch := httptest.NewRecorder()
	h.ServeHTTP(recSearch, reqSearch)
	if recSearch.Code != http.StatusOK {
		t.Fatalf("org search status=%d", recSearch.Code)
	}

	reqNavZH := httptest.NewRequest(http.MethodGet, "/ui/nav", nil)
	reqNavZH.Host = "localhost:8080"
	reqNavZH.AddCookie(session)
	reqNavZH.AddCookie(&http.Cookie{Name: "lang", Value: "zh"})
	recNavZH := httptest.NewRecorder()
	h.ServeHTTP(recNavZH, reqNavZH)
	if recNavZH.Code != http.StatusOK {
		t.Fatalf("nav zh status=%d", recNavZH.Code)
	}

	reqTopbarZH := httptest.NewRequest(http.MethodGet, "/ui/topbar", nil)
	reqTopbarZH.Host = "localhost:8080"
	reqTopbarZH.AddCookie(session)
	reqTopbarZH.AddCookie(&http.Cookie{Name: "lang", Value: "zh"})
	recTopbarZH := httptest.NewRecorder()
	h.ServeHTTP(recTopbarZH, reqTopbarZH)
	if recTopbarZH.Code != http.StatusOK {
		t.Fatalf("topbar zh status=%d", recTopbarZH.Code)
	}

	reqLangNoRef := httptest.NewRequest(http.MethodGet, "/lang/en", nil)
	reqLangNoRef.Host = "localhost:8080"
	recLangNoRef := httptest.NewRecorder()
	h.ServeHTTP(recLangNoRef, reqLangNoRef)
	if recLangNoRef.Code != http.StatusFound {
		t.Fatalf("lang status=%d", recLangNoRef.Code)
	}

	reqLangWithRef := httptest.NewRequest(http.MethodGet, "/lang/zh", nil)
	reqLangWithRef.Host = "localhost:8080"
	reqLangWithRef.Header.Set("Referer", "/app")
	recLangWithRef := httptest.NewRecorder()
	h.ServeHTTP(recLangWithRef, reqLangWithRef)
	if recLangWithRef.Code != http.StatusFound {
		t.Fatalf("lang status=%d", recLangWithRef.Code)
	}

	oldSession := session
	reqLogout := httptest.NewRequest(http.MethodPost, "/logout", nil)
	reqLogout.Host = "localhost:8080"
	reqLogout.AddCookie(oldSession)
	recLogout := httptest.NewRecorder()
	h.ServeHTTP(recLogout, reqLogout)
	if recLogout.Code != http.StatusFound {
		t.Fatalf("logout status=%d", recLogout.Code)
	}
	var cleared bool
	for _, c := range recLogout.Result().Cookies() {
		if c.Name == sidCookieName && c.MaxAge < 0 {
			cleared = true
			break
		}
	}
	if !cleared {
		t.Fatalf("expected %s cookie cleared", sidCookieName)
	}

	reqAppOldSession := httptest.NewRequest(http.MethodGet, "/app", nil)
	reqAppOldSession.Host = "localhost:8080"
	reqAppOldSession.AddCookie(oldSession)
	recAppOldSession := httptest.NewRecorder()
	h.ServeHTTP(recAppOldSession, reqAppOldSession)
	if recAppOldSession.Code != http.StatusFound {
		t.Fatalf("app (old session) status=%d", recAppOldSession.Code)
	}

	reqLoginPost2 := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=tenant-admin%40example.invalid&password=pw"))
	reqLoginPost2.Host = "localhost:8080"
	reqLoginPost2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recLoginPost2 := httptest.NewRecorder()
	h.ServeHTTP(recLoginPost2, reqLoginPost2)
	if recLoginPost2.Code != http.StatusFound {
		t.Fatalf("login post (2) status=%d", recLoginPost2.Code)
	}
	session = nil
	for _, c := range recLoginPost2.Result().Cookies() {
		if c.Name == sidCookieName {
			session = c
			break
		}
	}
	if session == nil || session.Value == "" {
		t.Fatalf("missing %s cookie (2)", sidCookieName)
	}

	reqPersonPost := httptest.NewRequest(http.MethodPost, "/person/persons?as_of=2026-01-01", strings.NewReader("pernr=1&display_name=A"))
	reqPersonPost.Host = "localhost:8080"
	reqPersonPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqPersonPost.AddCookie(session)
	recPersonPost := httptest.NewRecorder()
	h.ServeHTTP(recPersonPost, reqPersonPost)
	if recPersonPost.Code != http.StatusSeeOther {
		t.Fatalf("person post status=%d", recPersonPost.Code)
	}

	reqPersonByPernr := httptest.NewRequest(http.MethodGet, "/person/api/persons:by-pernr?pernr=1", nil)
	reqPersonByPernr.Host = "localhost:8080"
	reqPersonByPernr.AddCookie(session)
	recPersonByPernr := httptest.NewRecorder()
	h.ServeHTTP(recPersonByPernr, reqPersonByPernr)
	if recPersonByPernr.Code != http.StatusOK {
		t.Fatalf("person by pernr status=%d", recPersonByPernr.Code)
	}
	var pResp struct {
		PersonUUID string `json:"person_uuid"`
	}
	if err := json.NewDecoder(recPersonByPernr.Body).Decode(&pResp); err != nil {
		t.Fatal(err)
	}
	if pResp.PersonUUID == "" {
		t.Fatal("missing person_uuid")
	}

	reqPersonOptions := httptest.NewRequest(http.MethodGet, "/person/api/persons:options?q=1&limit=10", nil)
	reqPersonOptions.Host = "localhost:8080"
	reqPersonOptions.AddCookie(session)
	recPersonOptions := httptest.NewRecorder()
	h.ServeHTTP(recPersonOptions, reqPersonOptions)
	if recPersonOptions.Code != http.StatusOK {
		t.Fatalf("person options status=%d", recPersonOptions.Code)
	}

	reqPosCreate := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"org_code":"ORG-1","job_profile_uuid":"jp1","name":"A"}`))
	reqPosCreate.Host = "localhost:8080"
	reqPosCreate.Header.Set("Content-Type", "application/json")
	reqPosCreate.AddCookie(session)
	recPosCreate := httptest.NewRecorder()
	h.ServeHTTP(recPosCreate, reqPosCreate)
	if recPosCreate.Code != http.StatusOK {
		t.Fatalf("positions api post status=%d", recPosCreate.Code)
	}
	var posResp struct {
		PositionUUID string `json:"position_uuid"`
	}
	if err := json.NewDecoder(recPosCreate.Body).Decode(&posResp); err != nil {
		t.Fatal(err)
	}
	if posResp.PositionUUID == "" {
		t.Fatal("missing position id")
	}

	reqPosList := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
	reqPosList.Host = "localhost:8080"
	reqPosList.AddCookie(session)
	recPosList := httptest.NewRecorder()
	h.ServeHTTP(recPosList, reqPosList)
	if recPosList.Code != http.StatusOK {
		t.Fatalf("positions api get status=%d", recPosList.Code)
	}

	reqAssignCreate := httptest.NewRequest(http.MethodPost, "/org/api/assignments?as_of=2026-01-01", strings.NewReader(`{"person_uuid":"`+pResp.PersonUUID+`","position_uuid":"`+posResp.PositionUUID+`"}`))
	reqAssignCreate.Host = "localhost:8080"
	reqAssignCreate.Header.Set("Content-Type", "application/json")
	reqAssignCreate.AddCookie(session)
	recAssignCreate := httptest.NewRecorder()
	h.ServeHTTP(recAssignCreate, reqAssignCreate)
	if recAssignCreate.Code != http.StatusOK {
		t.Fatalf("assignments api post status=%d", recAssignCreate.Code)
	}

	reqAssignList := httptest.NewRequest(http.MethodGet, "/org/api/assignments?as_of=2026-01-01&person_uuid="+pResp.PersonUUID, nil)
	reqAssignList.Host = "localhost:8080"
	reqAssignList.AddCookie(session)
	recAssignList := httptest.NewRecorder()
	h.ServeHTTP(recAssignList, reqAssignList)
	if recAssignList.Code != http.StatusOK {
		t.Fatalf("assignments api get status=%d", recAssignList.Code)
	}

	reqPosUIPost := httptest.NewRequest(http.MethodPost, "/org/positions?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-02&org_code=ORG-1&job_profile_uuid=jp1&name=A"))
	reqPosUIPost.Host = "localhost:8080"
	reqPosUIPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqPosUIPost.AddCookie(session)
	recPosUIPost := httptest.NewRecorder()
	h.ServeHTTP(recPosUIPost, reqPosUIPost)
	if recPosUIPost.Code != http.StatusSeeOther {
		t.Fatalf("positions ui post status=%d", recPosUIPost.Code)
	}

	reqPosUIGet := httptest.NewRequest(http.MethodGet, "/org/positions?as_of=2026-01-01", nil)
	reqPosUIGet.Host = "localhost:8080"
	reqPosUIGet.AddCookie(session)
	recPosUIGet := httptest.NewRecorder()
	h.ServeHTTP(recPosUIGet, reqPosUIGet)
	if recPosUIGet.Code != http.StatusOK {
		t.Fatalf("positions ui get status=%d", recPosUIGet.Code)
	}

	reqAssignUIPost := httptest.NewRequest(http.MethodPost, "/org/assignments?as_of=2026-01-01", strings.NewReader("effective_date=2026-01-02&pernr=1&position_uuid="+posResp.PositionUUID))
	reqAssignUIPost.Host = "localhost:8080"
	reqAssignUIPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqAssignUIPost.AddCookie(session)
	recAssignUIPost := httptest.NewRecorder()
	h.ServeHTTP(recAssignUIPost, reqAssignUIPost)
	if recAssignUIPost.Code != http.StatusSeeOther {
		t.Fatalf("assignments ui post status=%d", recAssignUIPost.Code)
	}

	_ = tr("en", "unknown")
	_ = tr("zh", "unknown")
	if got := tr("zh", "shared_readonly"); got != "共享/只读" {
		t.Fatalf("shared_readonly zh=%q", got)
	}
	if got := tr("zh", "tenant_owned"); got != "租户" {
		t.Fatalf("tenant_owned zh=%q", got)
	}

	rNoCookie := &http.Request{Header: http.Header{}}
	_ = lang(rNoCookie)
	rOther := &http.Request{Header: http.Header{}}
	rOther.AddCookie(&http.Cookie{Name: "lang", Value: "fr"})
	_ = lang(rOther)
}

func TestNewHandler_InternalAPIRoutes(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "shadow")

	orgStore := newOrgUnitMemoryStore()
	tenantID := "00000000-0000-0000-0000-000000000001"
	node, err := orgStore.CreateNodeCurrent(context.Background(), tenantID, "2026-01-01", "ORG1", "Org1", "", false)
	if err != nil {
		t.Fatal(err)
	}
	setidStore := newSetIDMemoryStore().(*setidMemoryStore)
	writeSvc := orgUnitWriteServiceStub{
		createFn: func(_ context.Context, _ string, req orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			return orgunittypes.OrgUnitResult{
				OrgID:         "10000002",
				OrgCode:       req.OrgCode,
				EffectiveDate: req.EffectiveDate,
			}, nil
		},
	}

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{
			KratosIdentityID: "00000000-0000-0000-0000-0000000000aa",
			Email:            "tenant-admin@example.invalid",
		}},
		OrgUnitStore:        orgStore,
		OrgUnitWriteService: writeSvc,
		SetIDStore:          setidStore,
	})
	if err != nil {
		t.Fatal(err)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=tenant-admin%40example.invalid&password=pw"))
	loginReq.Host = "localhost:8080"
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusFound {
		t.Fatalf("login status=%d", loginRec.Code)
	}
	var session *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == sidCookieName {
			session = c
			break
		}
	}
	if session == nil {
		t.Fatal("missing session cookie")
	}

	postJSON := func(path string, body string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Host = "localhost:8080"
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		req.AddCookie(session)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	getJSON := func(path string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Host = "localhost:8080"
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		req.AddCookie(session)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	recSet := postJSON("/org/api/setids", `{"setid":"A0001","name":"Default","request_code":"r1"}`, nil)
	if recSet.Code != http.StatusCreated {
		t.Fatalf("setid status=%d", recSet.Code)
	}

	recBind := postJSON("/org/api/setid-bindings", `{"org_code":"`+node.OrgCode+`","setid":"A0001","effective_date":"2026-01-01","request_code":"r2"}`, nil)
	if recBind.Code != http.StatusCreated {
		t.Fatalf("binding status=%d", recBind.Code)
	}

	recGlobal := postJSON("/org/api/global-setids", `{"name":"Shared","request_code":"r3"}`, map[string]string{"X-Actor-Scope": "saas"})
	if recGlobal.Code != http.StatusCreated {
		t.Fatalf("global setid status=%d", recGlobal.Code)
	}
	recGlobalList := getJSON("/org/api/global-setids", nil)
	if recGlobalList.Code != http.StatusOK {
		t.Fatalf("global setid list status=%d", recGlobalList.Code)
	}

	recBU := postJSON("/org/api/org-units/set-business-unit", `{"org_code":"`+node.OrgCode+`","effective_date":"2026-01-01","is_business_unit":true,"request_code":"r4"}`, nil)
	if recBU.Code != http.StatusOK {
		t.Fatalf("set business unit status=%d", recBU.Code)
	}

	recOrgList := getJSON("/org/api/org-units?as_of=2026-01-01", nil)
	if recOrgList.Code != http.StatusOK {
		t.Fatalf("org units list status=%d", recOrgList.Code)
	}

	recOrgDetails := getJSON("/org/api/org-units/details?org_code="+node.OrgCode+"&as_of=2026-01-01", nil)
	if recOrgDetails.Code != http.StatusOK {
		t.Fatalf("org units details status=%d body=%s", recOrgDetails.Code, recOrgDetails.Body.String())
	}

	recOrgFieldDefs := getJSON("/org/api/org-units/field-definitions", nil)
	if recOrgFieldDefs.Code != http.StatusOK {
		t.Fatalf("org units field-definitions status=%d body=%s", recOrgFieldDefs.Code, recOrgFieldDefs.Body.String())
	}

	// Memory store does not implement the configs/options/capabilities interfaces; the routes should exist and fail-closed.
	recOrgFieldConfigs := getJSON("/org/api/org-units/field-configs?as_of=2026-01-01", nil)
	if recOrgFieldConfigs.Code != http.StatusInternalServerError {
		t.Fatalf("org units field-configs status=%d body=%s", recOrgFieldConfigs.Code, recOrgFieldConfigs.Body.String())
	}

	recOrgFieldEnable := postJSON("/org/api/org-units/field-configs", `{"field_key":"org_type","enabled_on":"2026-01-01","request_code":"rfc1"}`, nil)
	if recOrgFieldEnable.Code != http.StatusInternalServerError {
		t.Fatalf("org units field-configs enable status=%d body=%s", recOrgFieldEnable.Code, recOrgFieldEnable.Body.String())
	}

	recOrgFieldDisable := postJSON("/org/api/org-units/field-configs:disable", `{"field_key":"org_type","disabled_on":"2026-02-01","request_code":"rfc2"}`, nil)
	if recOrgFieldDisable.Code != http.StatusInternalServerError {
		t.Fatalf("org units field-configs disable status=%d body=%s", recOrgFieldDisable.Code, recOrgFieldDisable.Body.String())
	}

	recOrgFieldOptions := getJSON("/org/api/org-units/fields:options?as_of=2026-01-01&field_key=org_type", nil)
	if recOrgFieldOptions.Code != http.StatusInternalServerError {
		t.Fatalf("org units fields options status=%d body=%s", recOrgFieldOptions.Code, recOrgFieldOptions.Body.String())
	}

	recOrgMutationCaps := getJSON("/org/api/org-units/mutation-capabilities?org_code="+node.OrgCode+"&effective_date=2026-01-01", nil)
	if recOrgMutationCaps.Code != http.StatusInternalServerError {
		t.Fatalf("org units mutation capabilities status=%d body=%s", recOrgMutationCaps.Code, recOrgMutationCaps.Body.String())
	}

	recOrgVersions := getJSON("/org/api/org-units/versions?org_code="+node.OrgCode, nil)
	if recOrgVersions.Code != http.StatusOK {
		t.Fatalf("org units versions status=%d body=%s", recOrgVersions.Code, recOrgVersions.Body.String())
	}

	recOrgAudit := getJSON("/org/api/org-units/audit?org_code="+node.OrgCode+"&limit=1", nil)
	if recOrgAudit.Code != http.StatusOK {
		t.Fatalf("org units audit status=%d body=%s", recOrgAudit.Code, recOrgAudit.Body.String())
	}

	recOrgSearch := getJSON("/org/api/org-units/search?query="+node.OrgCode+"&as_of=2026-01-01", nil)
	if recOrgSearch.Code != http.StatusOK {
		t.Fatalf("org units search status=%d body=%s", recOrgSearch.Code, recOrgSearch.Body.String())
	}

	recOrgCreate := postJSON("/org/api/org-units", `{"org_code":"ORG2","name":"Org2","effective_date":"2026-01-01"}`, nil)
	if recOrgCreate.Code != http.StatusCreated {
		t.Fatalf("org units create status=%d", recOrgCreate.Code)
	}

	recOrgRename := postJSON("/org/api/org-units/rename", `{"org_code":"ORG2","new_name":"Org2b","effective_date":"2026-01-01"}`, nil)
	if recOrgRename.Code != http.StatusOK {
		t.Fatalf("org units rename status=%d", recOrgRename.Code)
	}

	recOrgMove := postJSON("/org/api/org-units/move", `{"org_code":"ORG2","new_parent_org_code":"ORG1","effective_date":"2026-01-01"}`, nil)
	if recOrgMove.Code != http.StatusOK {
		t.Fatalf("org units move status=%d", recOrgMove.Code)
	}

	recOrgDisable := postJSON("/org/api/org-units/disable", `{"org_code":"ORG2","effective_date":"2026-01-01"}`, nil)
	if recOrgDisable.Code != http.StatusOK {
		t.Fatalf("org units disable status=%d", recOrgDisable.Code)
	}

	recOrgEnable := postJSON("/org/api/org-units/enable", `{"org_code":"ORG2","effective_date":"2026-01-01"}`, nil)
	if recOrgEnable.Code != http.StatusOK {
		t.Fatalf("org units enable status=%d", recOrgEnable.Code)
	}

	recOrgCorrect := postJSON("/org/api/org-units/corrections", `{"org_code":"ORG2","effective_date":"2026-01-01","request_id":"r9","patch":{}}`, nil)
	if recOrgCorrect.Code != http.StatusOK {
		t.Fatalf("org units corrections status=%d", recOrgCorrect.Code)
	}

	recOrgStatusCorrect := postJSON("/org/api/org-units/status-corrections", `{"org_code":"ORG2","effective_date":"2026-01-01","target_status":"disabled","request_id":"r9s"}`, nil)
	if recOrgStatusCorrect.Code != http.StatusOK {
		t.Fatalf("org units status corrections status=%d", recOrgStatusCorrect.Code)
	}

	recOrgRescind := postJSON("/org/api/org-units/rescinds", `{"org_code":"ORG2","effective_date":"2026-01-01","request_id":"r10","reason":"bad-data"}`, nil)
	if recOrgRescind.Code != http.StatusOK {
		t.Fatalf("org units rescinds status=%d", recOrgRescind.Code)
	}

	recOrgRescindOrg := postJSON("/org/api/org-units/rescinds/org", `{"org_code":"ORG2","request_id":"r11","reason":"bad-org"}`, nil)
	if recOrgRescindOrg.Code != http.StatusOK {
		t.Fatalf("org units rescinds org status=%d", recOrgRescindOrg.Code)
	}

	reqView := httptest.NewRequest(http.MethodGet, "/org/nodes/view?effective_date=2026-01-01&org_id="+node.ID, nil)
	reqView.Host = "localhost:8080"
	reqView.Header.Set("HX-Request", "true")
	reqView.AddCookie(session)
	recView := httptest.NewRecorder()
	h.ServeHTTP(recView, reqView)
	if recView.Code != http.StatusOK {
		t.Fatalf("org nodes view status=%d", recView.Code)
	}
}

func TestNewHandlerWithOptions_DefaultOrgUnitSnapshotStoreFromPGStore(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    newOrgUnitPGStore(&fakeBeginner{}),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.Host = "localhost:8080"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestNewHandlerWithOptions_DefaultPaths(t *testing.T) {
	t.Cleanup(func() {
		_ = os.Unsetenv("ALLOWLIST_PATH")
	})
	_ = os.Unsetenv("ALLOWLIST_PATH")

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestNewHandlerWithOptions_AuthzLoadError(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	if err := os.Setenv("AUTHZ_MODEL_PATH", "no-such-model.conf"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("AUTHZ_MODEL_PATH") })

	_, err = NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    newOrgUnitMemoryStore(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandlerWithOptions_PoolDSNError(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	if err := os.Setenv("DATABASE_URL", "%%%"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("DATABASE_URL") })

	_, err = NewHandlerWithOptions(HandlerOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandlerWithOptions_DefaultStaffingStores(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    newOrgUnitMemoryStore(),
		PositionStore: positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if h == nil {
		t.Fatal("nil handler")
	}

	h2, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    newOrgUnitMemoryStore(),
		AssignmentStore: assignmentStoreStub{
			listFn: func(context.Context, string, string, string) ([]Assignment, error) { return nil, nil },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if h2 == nil {
		t.Fatal("nil handler")
	}

	h3, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    newOrgUnitPGStore(&fakeBeginner{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if h3 == nil {
		t.Fatal("nil handler")
	}

	h4, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    newOrgUnitPGStore(&fakeBeginner{}),
		PositionStore: positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if h4 == nil {
		t.Fatal("nil handler")
	}
}

type orgSnapshotStoreStub struct{}

func (orgSnapshotStoreStub) GetSnapshot(context.Context, string, string) ([]OrgUnitSnapshotRow, error) {
	return nil, nil
}
func (orgSnapshotStoreStub) CreateOrgUnit(context.Context, string, string, string, string) (string, error) {
	return "10000001", nil
}

func TestNewHandlerWithOptions_UsesProvidedStores(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	staffingStore := newStaffingMemoryStore()

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    newOrgUnitMemoryStore(),
		OrgUnitSnapshot: orgSnapshotStoreStub{},
		SetIDStore:      newSetIDMemoryStore(),
		JobCatalogStore: newJobCatalogMemoryStore(),
		PersonStore:     newPersonMemoryStore(),
		PositionStore:   staffingStore,
		AssignmentStore: staffingStore,
	})
	if err != nil {
		t.Fatal(err)
	}
	if h == nil {
		t.Fatal("nil handler")
	}
}

func TestNewHandlerWithOptions_WithProvidedStores(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    newOrgUnitMemoryStore(),
		OrgUnitSnapshot: nil,
		SetIDStore:      newSetIDMemoryStore(),
		JobCatalogStore: newJobCatalogMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestNewHandlerWithOptions_LoadAuthorizerError(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "")

	_, err = NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    newOrgUnitMemoryStore(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandlerWithOptions_DefaultAllowlistPath_NotFound(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	_ = os.Unsetenv("ALLOWLIST_PATH")

	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	_, err = NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    newOrgUnitMemoryStore(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandlerWithOptions_DefaultPGStore_DoesNotRequireDBAtBuildTime(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	t.Setenv("DATABASE_URL", "postgres://app:app@localhost:5432/bugs_and_blossoms?sslmode=disable")

	h, err := NewHandlerWithOptions(HandlerOptions{})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestNewHandlerWithOptions_BadDatabaseURL(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	t.Setenv("DATABASE_URL", "postgres://%zz")

	_, err = NewHandlerWithOptions(HandlerOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPathHasPrefixSegment(t *testing.T) {
	if !pathHasPrefixSegment("/assets", "/assets") {
		t.Fatal("expected true")
	}
	if pathHasPrefixSegment("/assetx", "/assets") {
		t.Fatal("expected false")
	}
}

func TestLoginPost_PrincipalError(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	old := sidRandReader
	t.Cleanup(func() { sidRandReader = old })
	sidRandReader = errReader{}

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{
			KratosIdentityID: "00000000-0000-0000-0000-0000000000aa",
			Email:            "tenant-admin@example.invalid",
		}},
		OrgUnitStore: newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=tenant-admin%40example.invalid&password=pw"))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestLoginPost_SessionError(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	old := sidRandReader
	t.Cleanup(func() { sidRandReader = old })
	sidRandReader = bytes.NewReader(bytes.Repeat([]byte{0xAB}, 16))

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{
			KratosIdentityID: "00000000-0000-0000-0000-0000000000aa",
			Email:            "tenant-admin@example.invalid",
		}},
		OrgUnitStore: newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=tenant-admin%40example.invalid&password=pw"))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestLoginPost_InvalidCredentials(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{
			err: errInvalidCredentials,
		},
		OrgUnitStore: newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=tenant-admin%40example.invalid&password=bad"))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestLoginPost_IdentityError(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{
			err: errors.New("boom"),
		},
		OrgUnitStore: newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=tenant-admin%40example.invalid&password=pw"))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestLoginPost_MissingFields(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{
			KratosIdentityID: "00000000-0000-0000-0000-0000000000aa",
			Email:            "tenant-admin@example.invalid",
		}},
		OrgUnitStore: newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=tenant-admin%40example.invalid"))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestLoginPost_BadForm(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{
			KratosIdentityID: "00000000-0000-0000-0000-0000000000aa",
			Email:            "tenant-admin@example.invalid",
		}},
		OrgUnitStore: newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=%zz&password=pw"))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestLoginPost_DefaultProviderConfigError(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	t.Setenv("KRATOS_PUBLIC_URL", "ftp://localhost:4433")

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		OrgUnitStore:    newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=tenant-admin%40example.invalid&password=pw"))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}
