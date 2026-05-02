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
	orgunitports "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/ports"
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

type orgUnitMemoryStoreWithWriteStore struct {
	*orgUnitMemoryStore
}

func (s orgUnitMemoryStoreWithWriteStore) SubmitEvent(context.Context, string, string, *int, string, string, json.RawMessage, string, string) (int64, error) {
	return 1, nil
}

func (s orgUnitMemoryStoreWithWriteStore) SubmitCorrection(context.Context, string, int, string, json.RawMessage, string, string) (string, error) {
	return "", nil
}

func (s orgUnitMemoryStoreWithWriteStore) SubmitStatusCorrection(context.Context, string, int, string, string, string, string) (string, error) {
	return "", nil
}

func (s orgUnitMemoryStoreWithWriteStore) SubmitRescindEvent(context.Context, string, int, string, string, string, string) (string, error) {
	return "", nil
}

func (s orgUnitMemoryStoreWithWriteStore) SubmitRescindOrg(context.Context, string, int, string, string, string) (int, error) {
	return 0, nil
}

func (s orgUnitMemoryStoreWithWriteStore) FindEventByUUID(context.Context, string, string) (orgunittypes.OrgUnitEvent, error) {
	return orgunittypes.OrgUnitEvent{}, orgunitports.ErrOrgEventNotFound
}

func (s orgUnitMemoryStoreWithWriteStore) FindEventByEffectiveDate(context.Context, string, int, string) (orgunittypes.OrgUnitEvent, error) {
	return orgunittypes.OrgUnitEvent{}, orgunitports.ErrOrgEventNotFound
}

func (s orgUnitMemoryStoreWithWriteStore) ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]orgUnitTenantFieldConfig, error) {
	return s.orgUnitMemoryStore.ListEnabledTenantFieldConfigsAsOf(ctx, tenantID, asOf)
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
	writeSvc := orgUnitWriteServiceStub{
		createFn: func(ctx context.Context, tenantID string, req orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
			var parentOrgNodeKey string
			if strings.TrimSpace(req.ParentOrgCode) != "" {
				var err error
				parentOrgNodeKey, err = orgStore.ResolveOrgNodeKeyByCode(ctx, tenantID, req.ParentOrgCode)
				if err != nil {
					return orgunittypes.OrgUnitResult{}, err
				}
			}
			if _, err := orgStore.CreateNodeCurrent(ctx, tenantID, req.EffectiveDate, req.OrgCode, req.Name, parentOrgNodeKey, req.IsBusinessUnit); err != nil {
				return orgunittypes.OrgUnitResult{}, err
			}
			return orgunittypes.OrgUnitResult{
				OrgCode:       req.OrgCode,
				EffectiveDate: req.EffectiveDate,
			}, nil
		},
		renameFn: func(ctx context.Context, tenantID string, req orgunitservices.RenameOrgUnitRequest) error {
			orgNodeKey, err := orgStore.ResolveOrgNodeKeyByCode(ctx, tenantID, req.OrgCode)
			if err != nil {
				return err
			}
			return orgStore.RenameNodeCurrent(ctx, tenantID, req.EffectiveDate, orgNodeKey, req.NewName)
		},
		moveFn: func(ctx context.Context, tenantID string, req orgunitservices.MoveOrgUnitRequest) error {
			orgNodeKey, err := orgStore.ResolveOrgNodeKeyByCode(ctx, tenantID, req.OrgCode)
			if err != nil {
				return err
			}
			var newParentOrgNodeKey string
			if strings.TrimSpace(req.NewParentOrgCode) != "" {
				newParentOrgNodeKey, err = orgStore.ResolveOrgNodeKeyByCode(ctx, tenantID, req.NewParentOrgCode)
				if err != nil {
					return err
				}
			}
			return orgStore.MoveNodeCurrent(ctx, tenantID, req.EffectiveDate, orgNodeKey, newParentOrgNodeKey)
		},
		disableFn: func(context.Context, string, orgunitservices.DisableOrgUnitRequest) error {
			return nil
		},
		enableFn: func(context.Context, string, orgunitservices.EnableOrgUnitRequest) error {
			return nil
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

	recDicts := getJSON("/iam/api/dicts?as_of=2026-01-01", nil)
	if recDicts.Code != http.StatusOK {
		t.Fatalf("dicts status=%d body=%s", recDicts.Code, recDicts.Body.String())
	}
	recDictValues := getJSON("/iam/api/dicts/values?dict_code=org_type&as_of=2026-01-01&status=all", nil)
	if recDictValues.Code != http.StatusOK {
		t.Fatalf("dict values status=%d body=%s", recDictValues.Code, recDictValues.Body.String())
	}
	recDictAudit := getJSON("/iam/api/dicts/values/audit?dict_code=org_type&code=10&limit=10", nil)
	if recDictAudit.Code != http.StatusOK {
		t.Fatalf("dict audit status=%d body=%s", recDictAudit.Code, recDictAudit.Body.String())
	}

	recBU := postJSON("/org/api/org-units/set-business-unit", `{"org_code":"`+node.OrgCode+`","effective_date":"2026-01-01","is_business_unit":true,"request_id":"r4"}`, nil)
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

	// Memory store does not implement field configs/options interfaces.
	recOrgFieldConfigs := getJSON("/org/api/org-units/field-configs?as_of=2026-01-01", nil)
	if recOrgFieldConfigs.Code != http.StatusInternalServerError {
		t.Fatalf("org units field-configs status=%d body=%s", recOrgFieldConfigs.Code, recOrgFieldConfigs.Body.String())
	}

	recOrgFieldEnable := postJSON("/org/api/org-units/field-configs", `{"field_key":"org_type","enabled_on":"2026-01-01","request_id":"rfc1"}`, nil)
	if recOrgFieldEnable.Code != http.StatusInternalServerError {
		t.Fatalf("org units field-configs enable status=%d body=%s", recOrgFieldEnable.Code, recOrgFieldEnable.Body.String())
	}

	recOrgFieldDisable := postJSON("/org/api/org-units/field-configs:disable", `{"field_key":"org_type","disabled_on":"2026-02-01","request_id":"rfc2"}`, nil)
	if recOrgFieldDisable.Code != http.StatusInternalServerError {
		t.Fatalf("org units field-configs disable status=%d body=%s", recOrgFieldDisable.Code, recOrgFieldDisable.Body.String())
	}

	recOrgFieldOptions := getJSON("/org/api/org-units/fields:options?as_of=2026-01-01&field_key=org_type", nil)
	if recOrgFieldOptions.Code != http.StatusInternalServerError {
		t.Fatalf("org units fields options status=%d body=%s", recOrgFieldOptions.Code, recOrgFieldOptions.Body.String())
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

	recOrgCreate := postJSON("/org/api/org-units", `{"org_code":"ORG2","name":"Org2","parent_org_code":"ORG1","effective_date":"2026-01-01"}`, nil)
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

func stringsReader(s string) *strings.Reader { return strings.NewReader(s) }

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}

func mustAllowlistPathFromWd(t *testing.T, wd string) string {
	t.Helper()
	return filepath.Clean(filepath.Join(wd, "..", "..", "config", "routing", "allowlist.yaml"))
}

func loginTenantAdminCookie(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	login := httptest.NewRequest(http.MethodPost, "http://localhost/iam/api/sessions", stringsReader(`{"email":"tenant-admin@example.invalid","password":"pw"}`))
	login.Host = "localhost"
	login.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, login)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("missing sid cookie")
	}
	return cookies[0]
}

func TestNewHandlerWithOptions_DictRoutes_AreWired(t *testing.T) {
	wd := mustGetwd(t)
	allowlistPath := mustAllowlistPathFromWd(t, wd)
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{
			KratosIdentityID: "00000000-0000-0000-0000-0000000000aa",
			Email:            "tenant-admin@example.invalid",
			RoleSlug:         "tenant-admin",
		}},
		OrgUnitStore: newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	sidCookie := loginTenantAdminCookie(t, h)
	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil)
		req.Host = "localhost"
		req.AddCookie(sidCookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	post := func(path string, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://localhost"+path, stringsReader(body))
		req.Host = "localhost"
		req.AddCookie(sidCookie)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := get("/iam/api/dicts?as_of=2026-01-01"); rec.Code != http.StatusOK {
		t.Fatalf("dicts status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := post("/iam/api/dicts", `{"dict_code":"expense_type","name":"Expense Type","enabled_on":"2026-01-01","request_id":"r-dict-create"}`); rec.Code != http.StatusCreated {
		t.Fatalf("dict create status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := post("/iam/api/dicts:disable", `{"dict_code":"expense_type","disabled_on":"2026-01-02","request_id":"r-dict-disable"}`); rec.Code != http.StatusOK {
		t.Fatalf("dict disable status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := get("/iam/api/dicts/values?dict_code=org_type&as_of=2026-01-01&status=all"); rec.Code != http.StatusOK {
		t.Fatalf("dict values status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := post("/iam/api/dicts/values", `{"dict_code":"org_type","code":"30","label":"X","enabled_on":"2026-01-01","request_id":"r1"}`); rec.Code != http.StatusConflict {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := post("/iam/api/dicts/values:disable", `{"dict_code":"org_type","code":"10","disabled_on":"2026-01-01","request_id":"r1"}`); rec.Code != http.StatusConflict {
		t.Fatalf("disable status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := post("/iam/api/dicts/values:correct", `{"dict_code":"org_type","code":"10","label":"X","correction_day":"2026-01-01","request_id":"r1"}`); rec.Code != http.StatusConflict {
		t.Fatalf("correct status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := get("/iam/api/dicts/values/audit?dict_code=org_type&code=10&limit=10"); rec.Code != http.StatusOK {
		t.Fatalf("audit status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := post("/iam/api/dicts:release:preview", `{"release_id":"r1","as_of":"2026-01-01"}`); rec.Code != http.StatusInternalServerError {
		t.Fatalf("release preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := post("/iam/api/dicts:release", `{"release_id":"r1","request_id":"req-1","as_of":"2026-01-01"}`); rec.Code != http.StatusInternalServerError {
		t.Fatalf("release status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNewHandlerWithOptions_CubeBoxCapabilitiesRouteIsInternalJSON(t *testing.T) {
	wd := mustGetwd(t)
	allowlistPath := mustAllowlistPathFromWd(t, wd)
	t.Setenv("ALLOWLIST_PATH", allowlistPath)

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{
			KratosIdentityID: "00000000-0000-0000-0000-0000000000aa",
			Email:            "tenant-admin@example.invalid",
			RoleSlug:         "tenant-admin",
		}},
		OrgUnitStore: newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	sidCookie := loginTenantAdminCookie(t, h)
	req := httptest.NewRequest(http.MethodGet, "http://localhost/internal/cubebox/capabilities", nil)
	req.Host = "localhost"
	req.AddCookie(sidCookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Result().Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type=%q body=%s", ct, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"conversation":{"read":true,"use":true}`) {
		t.Fatalf("unexpected body=%s", rec.Body.String())
	}
}

func TestNewHandlerWithOptions_OrgUnitFieldConfigRoutes_AreWired(t *testing.T) {
	wd := mustGetwd(t)
	allowlistPath := mustAllowlistPathFromWd(t, wd)
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{
			KratosIdentityID: "00000000-0000-0000-0000-0000000000aa",
			Email:            "tenant-admin@example.invalid",
			RoleSlug:         "tenant-admin",
		}},
		OrgUnitStore: newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	sidCookie := loginTenantAdminCookie(t, h)
	req := httptest.NewRequest(http.MethodGet, "http://localhost/org/api/org-units/field-configs:enable-candidates?enabled_on=2026-01-01", nil)
	req.Host = "localhost"
	req.AddCookie(sidCookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNewHandlerWithOptions_OrgUnitFieldConfigGovernanceRoutes_AreWired(t *testing.T) {
	wd := mustGetwd(t)
	allowlistPath := mustAllowlistPathFromWd(t, wd)
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{
			KratosIdentityID: "00000000-0000-0000-0000-0000000000ab",
			Email:            "tenant-admin@example.invalid",
			RoleSlug:         "tenant-admin",
		}},
		OrgUnitStore: newOrgUnitMemoryStore(),
	})
	if err != nil {
		t.Fatal(err)
	}

	sidCookie := loginTenantAdminCookie(t, h)
	check := func(method, path, body string, want int) {
		t.Helper()
		req := httptest.NewRequest(method, "http://localhost"+path, stringsReader(body))
		req.Host = "localhost"
		req.AddCookie(sidCookie)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s status=%d body=%s", method, path, rec.Code, rec.Body.String())
		}
	}

	check(http.MethodPost, "/org/api/org-units/field-policies", `{"field_key":"","enabled_on":"","request_id":""}`, http.StatusNotFound)
	check(http.MethodPost, "/org/api/org-units/field-policies:disable", `{"field_key":"","disabled_on":"","request_id":""}`, http.StatusNotFound)
	check(http.MethodGet, "/org/api/org-units/field-policies:resolve-preview?as_of=2026-01-01", "", http.StatusNotFound)
}

func TestNewHandlerWithOptions_OrgUnitWriteRoutes_AreWired(t *testing.T) {
	wd := mustGetwd(t)
	allowlistPath := mustAllowlistPathFromWd(t, wd)
	t.Setenv("ALLOWLIST_PATH", allowlistPath)
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver: localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{
			KratosIdentityID: "00000000-0000-0000-0000-0000000000aa",
			Email:            "tenant-admin@example.invalid",
			RoleSlug:         "tenant-admin",
		}},
		OrgUnitStore:        newOrgUnitMemoryStore(),
		OrgUnitWriteService: fakeOrgUnitWriteService{},
	})
	if err != nil {
		t.Fatal(err)
	}

	sidCookie := loginTenantAdminCookie(t, h)
	t.Run("write", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://localhost/org/api/org-units/write", stringsReader(`{"intent":"create_org","org_code":"ROOT","effective_date":"2026-01-01","request_id":"r1","patch":{"name":"Root A"}}`))
		req.Host = "localhost"
		req.AddCookie(sidCookie)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Fatalf("expected route to be wired, got 404")
		}
	})
}

func TestNewHandlerWithOptions_HealthRemainsAvailableForStores(t *testing.T) {
	wd := mustGetwd(t)
	t.Setenv("ALLOWLIST_PATH", mustAllowlistPathFromWd(t, wd))
	t.Setenv("AUTHZ_MODE", "disabled")
	t.Setenv("AUTHZ_UNSAFE_ALLOW_DISABLED", "1")

	h, err := NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{KratosIdentityID: "00000000-0000-0000-0000-000000000001", Email: "tenant-admin@example.invalid", RoleSlug: "tenant-admin"}},
		OrgUnitStore:     &orgUnitPGStore{},
	})
	if err != nil {
		t.Fatalf("new handler failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost/health", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status=%d body=%s", rec.Code, rec.Body.String())
	}

	memoryWithWrite := orgUnitMemoryStoreWithWriteStore{orgUnitMemoryStore: newOrgUnitMemoryStore()}
	h, err = NewHandlerWithOptions(HandlerOptions{
		TenancyResolver:  localTenancyResolver(),
		IdentityProvider: staticIdentityProvider{ident: authenticatedIdentity{KratosIdentityID: "00000000-0000-0000-0000-000000000001", Email: "tenant-admin@example.invalid", RoleSlug: "tenant-admin"}},
		OrgUnitStore:     memoryWithWrite,
	})
	if err != nil {
		t.Fatalf("new handler with write store failed: %v", err)
	}

}
