package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestMustNewHandler_PanicsOnBadPath(t *testing.T) {
	if err := os.Setenv("ALLOWLIST_PATH", "no-such-file.yaml"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = MustNewHandler()
}

func TestMustNewHandler_Success(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	h := MustNewHandler()
	if h == nil {
		t.Fatal("nil handler")
	}
}

func TestDefaultAllowlistPath_Errors(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	_, err = defaultAllowlistPath()
	if err == nil {
		t.Fatal("expected error")
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

func TestNewHandler_TenantsError(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	if err := os.Setenv("ALLOWLIST_PATH", allowlistPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })

	if err := os.Setenv("TENANTS_PATH", "no-such-file.yaml"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("TENANTS_PATH") })

	_, err = NewHandler()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandler_DefaultAllowlistNotFound(t *testing.T) {
	t.Cleanup(func() { _ = os.Unsetenv("ALLOWLIST_PATH") })
	t.Cleanup(func() { _ = os.Unsetenv("TENANTS_PATH") })

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tenantsPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "tenants.yaml"))
	if err := os.Setenv("TENANTS_PATH", tenantsPath); err != nil {
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

	tenantsPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "tenants.yaml"))
	if err := os.Setenv("TENANTS_PATH", tenantsPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("TENANTS_PATH") })

	h, err := NewHandlerWithOptions(HandlerOptions{OrgUnitStore: newOrgUnitMemoryStore()})
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
	if body := recLogin.Body.String(); strings.Contains(body, `hx-get="/ui/nav"`) || strings.Contains(body, `hx-get="/ui/topbar"`) {
		t.Fatalf("unexpected hx-get in login body: %q", body)
	}

	reqAppNoSession := httptest.NewRequest(http.MethodGet, "/app", nil)
	reqAppNoSession.Host = "localhost:8080"
	recAppNoSession := httptest.NewRecorder()
	h.ServeHTTP(recAppNoSession, reqAppNoSession)
	if recAppNoSession.Code != http.StatusFound {
		t.Fatalf("app (no session) status=%d", recAppNoSession.Code)
	}

	reqLoginPost := httptest.NewRequest(http.MethodPost, "/login", nil)
	reqLoginPost.Host = "localhost:8080"
	recLoginPost := httptest.NewRecorder()
	h.ServeHTTP(recLoginPost, reqLoginPost)
	if recLoginPost.Code != http.StatusFound {
		t.Fatalf("login post status=%d", recLoginPost.Code)
	}
	session := recLoginPost.Result().Cookies()[0]

	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	reqRoot.Host = "localhost:8080"
	reqRoot.AddCookie(session)
	recRoot := httptest.NewRecorder()
	h.ServeHTTP(recRoot, reqRoot)
	if recRoot.Code != http.StatusFound {
		t.Fatalf("root status=%d", recRoot.Code)
	}

	protected := []string{
		"/app",
		"/app/home",
		"/ui/flash",
		"/ui/nav",
		"/ui/topbar",
		"/org/nodes",
		"/org/snapshot",
		"/org/job-catalog",
		"/org/positions",
		"/org/assignments",
		"/person/persons",
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

	reqOrgSnapshotPost := httptest.NewRequest(http.MethodPost, "/org/snapshot", strings.NewReader("name=A"))
	reqOrgSnapshotPost.Host = "localhost:8080"
	reqOrgSnapshotPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqOrgSnapshotPost.AddCookie(session)
	recOrgSnapshotPost := httptest.NewRecorder()
	h.ServeHTTP(recOrgSnapshotPost, reqOrgSnapshotPost)
	if recOrgSnapshotPost.Code != http.StatusOK {
		t.Fatalf("org snapshot post status=%d", recOrgSnapshotPost.Code)
	}

	reqCreate := httptest.NewRequest(http.MethodPost, "/org/nodes", strings.NewReader("name=NodeA"))
	reqCreate.Host = "localhost:8080"
	reqCreate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqCreate.AddCookie(session)
	recCreate := httptest.NewRecorder()
	h.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusSeeOther {
		t.Fatalf("org create status=%d", recCreate.Code)
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

	reqLogout := httptest.NewRequest(http.MethodPost, "/logout", nil)
	reqLogout.Host = "localhost:8080"
	reqLogout.AddCookie(session)
	recLogout := httptest.NewRecorder()
	h.ServeHTTP(recLogout, reqLogout)
	if recLogout.Code != http.StatusFound {
		t.Fatalf("logout status=%d", recLogout.Code)
	}

	_ = tr("en", "unknown")
	_ = tr("zh", "unknown")

	rNoCookie := &http.Request{Header: http.Header{}}
	_ = lang(rNoCookie)
	rOther := &http.Request{Header: http.Header{}}
	rOther.AddCookie(&http.Cookie{Name: "lang", Value: "fr"})
	_ = lang(rOther)
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

	tenantsPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "tenants.yaml"))
	if err := os.Setenv("TENANTS_PATH", tenantsPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("TENANTS_PATH") })

	h, err := NewHandlerWithOptions(HandlerOptions{
		OrgUnitStore: newOrgUnitPGStore(&fakeBeginner{}),
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
		_ = os.Unsetenv("TENANTS_PATH")
	})
	_ = os.Unsetenv("ALLOWLIST_PATH")
	_ = os.Unsetenv("TENANTS_PATH")

	h, err := NewHandlerWithOptions(HandlerOptions{OrgUnitStore: newOrgUnitMemoryStore()})
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

func TestNewHandlerWithOptions_DefaultAllowlistPath_NotFound(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tenantsPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "tenants.yaml"))
	if err := os.Setenv("TENANTS_PATH", tenantsPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("TENANTS_PATH") })

	_ = os.Unsetenv("ALLOWLIST_PATH")

	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	_, err = NewHandlerWithOptions(HandlerOptions{OrgUnitStore: newOrgUnitMemoryStore()})
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

	tenantsPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "tenants.yaml"))
	if err := os.Setenv("TENANTS_PATH", tenantsPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("TENANTS_PATH") })

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

	tenantsPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "tenants.yaml"))
	if err := os.Setenv("TENANTS_PATH", tenantsPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("TENANTS_PATH") })

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
