package server

import (
	"bytes"
	"context"
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

	req := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var sidCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sidCookieName && c.Value != "" {
			sidCookie = c
			break
		}
	}
	if sidCookie == nil {
		t.Fatalf("missing %s cookie", sidCookieName)
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

	loginReq := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	loginReq.Host = "localhost:8080"
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
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

func TestNewHandler_RouteEntrypointsAndLogout(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	allowlistPath := filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{Email: "tenant-admin@example.invalid", KratosIdentityID: "kid1", RoleSlug: "tenant-admin"}},
		OrgUnitStore:     newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	loginReq.Host = "localhost:8080"
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login status=%d body=%q", loginRec.Code, loginRec.Body.String())
	}

	var sidCookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == sidCookieName && c.Value != "" {
			sidCookie = c
			break
		}
	}
	if sidCookie == nil {
		t.Fatal("missing sid cookie")
	}

	rootReq := httptest.NewRequest(http.MethodGet, "/", nil)
	rootReq.Host = "localhost:8080"
	rootReq.AddCookie(sidCookie)
	rootRec := httptest.NewRecorder()
	h.ServeHTTP(rootRec, rootReq)
	if rootRec.Code != http.StatusFound {
		t.Fatalf("root status=%d", rootRec.Code)
	}
	if got := rootRec.Result().Header.Get("Location"); got != "/app" {
		t.Fatalf("root location=%q", got)
	}

	appReq := httptest.NewRequest(http.MethodGet, "/app", nil)
	appReq.Host = "localhost:8080"
	appReq.AddCookie(sidCookie)
	appRec := httptest.NewRecorder()
	h.ServeHTTP(appRec, appReq)
	if appRec.Code != http.StatusOK {
		t.Fatalf("app status=%d body=%q", appRec.Code, appRec.Body.String())
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/org/api/positions", body: ""},
		{method: http.MethodPost, path: "/org/api/positions", body: "{}"},
		{method: http.MethodGet, path: "/org/api/positions:options", body: ""},
		{method: http.MethodGet, path: "/org/api/assignments", body: ""},
		{method: http.MethodPost, path: "/org/api/assignments", body: "{}"},
		{method: http.MethodGet, path: "/org/api/setids", body: ""},
		{method: http.MethodGet, path: "/org/api/setid-bindings", body: ""},
		{method: http.MethodGet, path: "/person/api/persons", body: ""},
		{method: http.MethodPost, path: "/person/api/persons", body: "{}"},
		{method: http.MethodGet, path: "/person/api/persons:options", body: ""},
		{method: http.MethodGet, path: "/person/api/persons:by-pernr", body: ""},
		{method: http.MethodGet, path: "/jobcatalog/api/catalog", body: ""},
		{method: http.MethodPost, path: "/jobcatalog/api/catalog/actions", body: "{}"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Host = "localhost:8080"
		req.AddCookie(sidCookie)
		if tc.method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed {
			t.Fatalf("%s %s status=%d body=%q", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/logout", nil)
	logoutReq.Host = "localhost:8080"
	logoutReq.AddCookie(sidCookie)
	logoutRec := httptest.NewRecorder()
	h.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusFound {
		t.Fatalf("logout status=%d body=%q", logoutRec.Code, logoutRec.Body.String())
	}
	if got := logoutRec.Result().Header.Get("Location"); got != "/app/login" {
		t.Fatalf("logout location=%q", got)
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

func TestUI_MUIOnly(t *testing.T) {
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
	if recAsset.Code != http.StatusNotFound {
		t.Fatalf("asset status=%d body=%s", recAsset.Code, recAsset.Body.String())
	}

	reqNoTenant := httptest.NewRequest(http.MethodGet, "/app/login", nil)
	reqNoTenant.Host = ""
	recNoTenant := httptest.NewRecorder()
	h.ServeHTTP(recNoTenant, reqNoTenant)
	if recNoTenant.Code != http.StatusNotFound {
		t.Fatalf("no-tenant status=%d", recNoTenant.Code)
	}

	reqBadTenant := httptest.NewRequest(http.MethodGet, "/app/login", nil)
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
	if recLogin.Code != http.StatusNotFound {
		t.Fatalf("login status=%d body=%s", recLogin.Code, recLogin.Body.String())
	}

	reqAppNoSession := httptest.NewRequest(http.MethodGet, "/app", nil)
	reqAppNoSession.Host = "localhost:8080"
	recAppNoSession := httptest.NewRecorder()
	h.ServeHTTP(recAppNoSession, reqAppNoSession)
	if recAppNoSession.Code != http.StatusFound {
		t.Fatalf("app (no session) status=%d", recAppNoSession.Code)
	}
	if loc := recAppNoSession.Result().Header.Get("Location"); loc != "/app/login" {
		t.Fatalf("unexpected redirect location=%q", loc)
	}

	reqAppLogin := httptest.NewRequest(http.MethodGet, "/app/login", nil)
	reqAppLogin.Host = "localhost:8080"
	recAppLogin := httptest.NewRecorder()
	h.ServeHTTP(recAppLogin, reqAppLogin)
	if recAppLogin.Code != http.StatusOK {
		t.Fatalf("app login status=%d", recAppLogin.Code)
	}
	if body := recAppLogin.Body.String(); !strings.Contains(body, `<div id="root"></div>`) {
		t.Fatalf("unexpected body=%q", body)
	}

	reqAPI := httptest.NewRequest(http.MethodGet, "/org/api/org-units?as_of=2026-01-01", nil)
	reqAPI.Host = "localhost:8080"
	recAPI := httptest.NewRecorder()
	h.ServeHTTP(recAPI, reqAPI)
	if recAPI.Code != http.StatusUnauthorized {
		t.Fatalf("api status=%d body=%s", recAPI.Code, recAPI.Body.String())
	}
	if ct := recAPI.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("api content-type=%q", ct)
	}
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

	loginReq := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	loginReq.Host = "localhost:8080"
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
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

	// Memory store does not implement field configs/options and mutation-capabilities interfaces;
	// append-capabilities is supported and should return OK.
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
	recOrgAppendCaps := getJSON("/org/api/org-units/append-capabilities?org_code="+node.OrgCode+"&effective_date=2026-01-01", nil)
	if recOrgAppendCaps.Code != http.StatusOK {
		t.Fatalf("org units append capabilities status=%d body=%s", recOrgAppendCaps.Code, recOrgAppendCaps.Body.String())
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

	req := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
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

	req := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
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

	req := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"bad"}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
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

	req := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
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

	req := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid"}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
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

	req := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader("email=%zz&password=pw"))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
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

	req := httptest.NewRequest(http.MethodPost, "/iam/api/sessions", strings.NewReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}
