package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type errSetIDStore struct{ err error }

func (s errSetIDStore) EnsureBootstrap(context.Context, string, string) error { return s.err }
func (s errSetIDStore) ListSetIDs(context.Context, string) ([]SetID, error)   { return nil, s.err }
func (s errSetIDStore) ListGlobalSetIDs(context.Context) ([]SetID, error)     { return nil, s.err }
func (s errSetIDStore) CreateSetID(context.Context, string, string, string, string, string, string) error {
	return s.err
}
func (s errSetIDStore) ListSetIDBindings(context.Context, string, string) ([]SetIDBindingRow, error) {
	return nil, s.err
}
func (s errSetIDStore) BindSetID(context.Context, string, string, string, string, string, string) error {
	return s.err
}
func (s errSetIDStore) CreateGlobalSetID(context.Context, string, string, string, string) error {
	return s.err
}
func (s errSetIDStore) ListScopeCodes(context.Context, string) ([]ScopeCode, error) {
	return nil, s.err
}
func (s errSetIDStore) CreateScopePackage(context.Context, string, string, string, string, string, string, string, string) (ScopePackage, error) {
	return ScopePackage{}, s.err
}
func (s errSetIDStore) DisableScopePackage(context.Context, string, string, string, string) (ScopePackage, error) {
	return ScopePackage{}, s.err
}
func (s errSetIDStore) ListScopePackages(context.Context, string, string) ([]ScopePackage, error) {
	return nil, s.err
}
func (s errSetIDStore) ListOwnedScopePackages(context.Context, string, string, string) ([]OwnedScopePackage, error) {
	return nil, s.err
}
func (s errSetIDStore) CreateScopeSubscription(context.Context, string, string, string, string, string, string, string, string) (ScopeSubscription, error) {
	return ScopeSubscription{}, s.err
}
func (s errSetIDStore) GetScopeSubscription(context.Context, string, string, string, string) (ScopeSubscription, error) {
	return ScopeSubscription{}, s.err
}
func (s errSetIDStore) CreateGlobalScopePackage(context.Context, string, string, string, string, string, string, string) (ScopePackage, error) {
	return ScopePackage{}, s.err
}
func (s errSetIDStore) ListGlobalScopePackages(context.Context, string) ([]ScopePackage, error) {
	return nil, s.err
}

type partialSetIDStore struct {
	listSetErr   error
	createSetErr error
	listBindErr  error
	bindErr      error
}

func (s partialSetIDStore) EnsureBootstrap(context.Context, string, string) error { return nil }
func (s partialSetIDStore) ListSetIDs(context.Context, string) ([]SetID, error) {
	if s.listSetErr != nil {
		return nil, s.listSetErr
	}
	return []SetID{{SetID: "DEFLT", Name: "Default", Status: "active"}}, nil
}
func (s partialSetIDStore) ListGlobalSetIDs(context.Context) ([]SetID, error) {
	return []SetID{{SetID: "SHARE", Name: "Shared", Status: "active", IsShared: true}}, nil
}
func (s partialSetIDStore) CreateSetID(context.Context, string, string, string, string, string, string) error {
	return s.createSetErr
}
func (s partialSetIDStore) ListSetIDBindings(context.Context, string, string) ([]SetIDBindingRow, error) {
	if s.listBindErr != nil {
		return nil, s.listBindErr
	}
	return []SetIDBindingRow{{OrgUnitID: "10000001", SetID: "DEFLT", ValidFrom: "2026-01-01"}}, nil
}
func (s partialSetIDStore) BindSetID(context.Context, string, string, string, string, string, string) error {
	return s.bindErr
}
func (s partialSetIDStore) ListScopeCodes(context.Context, string) ([]ScopeCode, error) {
	return nil, nil
}
func (s partialSetIDStore) CreateScopePackage(context.Context, string, string, string, string, string, string, string, string) (ScopePackage, error) {
	return ScopePackage{}, nil
}
func (s partialSetIDStore) DisableScopePackage(context.Context, string, string, string, string) (ScopePackage, error) {
	return ScopePackage{}, nil
}
func (s partialSetIDStore) ListScopePackages(context.Context, string, string) ([]ScopePackage, error) {
	return nil, nil
}
func (s partialSetIDStore) ListOwnedScopePackages(context.Context, string, string, string) ([]OwnedScopePackage, error) {
	return nil, nil
}
func (s partialSetIDStore) CreateScopeSubscription(context.Context, string, string, string, string, string, string, string, string) (ScopeSubscription, error) {
	return ScopeSubscription{}, nil
}
func (s partialSetIDStore) GetScopeSubscription(context.Context, string, string, string, string) (ScopeSubscription, error) {
	return ScopeSubscription{}, nil
}
func (s partialSetIDStore) CreateGlobalScopePackage(context.Context, string, string, string, string, string, string, string) (ScopePackage, error) {
	return ScopePackage{}, nil
}
func (s partialSetIDStore) ListGlobalScopePackages(context.Context, string) ([]ScopePackage, error) {
	return nil, nil
}
func (s partialSetIDStore) CreateGlobalSetID(context.Context, string, string, string, string) error {
	return nil
}

type scopeTestStore struct {
	partialSetIDStore
	scopes             []ScopeCode
	scopePackages      map[string][]ScopePackage
	listScopeErr       error
	listScopePkgErr    error
	createScopePkgErr  error
	disableScopePkgErr error
}

func (s scopeTestStore) ListScopeCodes(context.Context, string) ([]ScopeCode, error) {
	if s.listScopeErr != nil {
		return nil, s.listScopeErr
	}
	if s.scopes != nil {
		return s.scopes, nil
	}
	return []ScopeCode{{ScopeCode: "jobcatalog", OwnerModule: "jobcatalog", ShareMode: "tenant-only", IsStable: true}}, nil
}

func (s scopeTestStore) ListScopePackages(_ context.Context, _ string, scopeCode string) ([]ScopePackage, error) {
	if s.listScopePkgErr != nil {
		return nil, s.listScopePkgErr
	}
	if s.scopePackages != nil {
		return s.scopePackages[scopeCode], nil
	}
	return nil, nil
}

func (s scopeTestStore) CreateScopePackage(context.Context, string, string, string, string, string, string, string, string) (ScopePackage, error) {
	if s.createScopePkgErr != nil {
		return ScopePackage{}, s.createScopePkgErr
	}
	return ScopePackage{
		PackageID:   "p1",
		ScopeCode:   "jobcatalog",
		PackageCode: "PKG1",
		OwnerSetID:  "A0001",
		Name:        "Pkg",
		Status:      "active",
	}, nil
}

func (s scopeTestStore) DisableScopePackage(context.Context, string, string, string, string) (ScopePackage, error) {
	if s.disableScopePkgErr != nil {
		return ScopePackage{}, s.disableScopePkgErr
	}
	return ScopePackage{
		PackageID: "p1",
		Status:    "disabled",
	}, nil
}

func TestSetIDMemoryStore_ListOwnedScopePackages(t *testing.T) {
	store := newSetIDMemoryStore().(*setidMemoryStore)
	tenantID := "t1"
	store.setids[tenantID] = map[string]SetID{
		"A0001": {SetID: "A0001", Name: "A", Status: "active"},
		"B0001": {SetID: "B0001", Name: "B", Status: "disabled"},
	}
	store.scopePackages[tenantID] = map[string]map[string]ScopePackage{
		"jobcatalog": {
			"pkg0": {PackageID: "pkg0", ScopeCode: "jobcatalog", PackageCode: "PKG0", OwnerSetID: "A0001", Name: "Pkg0", Status: "active"},
			"pkg1": {PackageID: "pkg1", ScopeCode: "jobcatalog", PackageCode: "PKG1", OwnerSetID: "A0001", Name: "Pkg1", Status: "active"},
			"pkg2": {PackageID: "pkg2", ScopeCode: "jobcatalog", PackageCode: "PKG2", OwnerSetID: "B0001", Name: "Pkg2", Status: "active"},
			"pkg3": {PackageID: "pkg3", ScopeCode: "jobcatalog", PackageCode: "PKG3", OwnerSetID: "A0001", Name: "Pkg3", Status: "disabled"},
		},
	}

	rows, err := store.ListOwnedScopePackages(context.Background(), tenantID, "jobcatalog", "2026-01-01")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[0].PackageID != "pkg0" || rows[0].EffectiveDate != "2026-01-01" {
		t.Fatalf("unexpected row[0]: %#v", rows[0])
	}
	if rows[1].PackageID != "pkg1" {
		t.Fatalf("unexpected row[1]: %#v", rows[1])
	}
}

type errOrgUnitStore struct{ err error }

func (s errOrgUnitStore) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	return nil, s.err
}
func (s errOrgUnitStore) CreateNodeCurrent(context.Context, string, string, string, string, bool) (OrgUnitNode, error) {
	return OrgUnitNode{}, s.err
}
func (s errOrgUnitStore) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return s.err
}
func (s errOrgUnitStore) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return s.err
}
func (s errOrgUnitStore) DisableNodeCurrent(context.Context, string, string, string) error {
	return s.err
}
func (s errOrgUnitStore) SetBusinessUnitCurrent(context.Context, string, string, string, bool, string) error {
	return s.err
}

type tableRows struct {
	idx  int
	rows [][]any
	err  error
}

func (r *tableRows) Close()                        {}
func (r *tableRows) Err() error                    { return r.err }
func (r *tableRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *tableRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *tableRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *tableRows) Scan(dest ...any) error {
	row := r.rows[r.idx-1]
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = row[i].(string)
		case *bool:
			*d = row[i].(bool)
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}
func (r *tableRows) Values() ([]any, error) { return nil, nil }
func (r *tableRows) RawValues() [][]byte    { return nil }
func (r *tableRows) Conn() *pgx.Conn        { return nil }

type scanErrRows struct {
	next bool
}

func (r *scanErrRows) Close()                        {}
func (r *scanErrRows) Err() error                    { return nil }
func (r *scanErrRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *scanErrRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *scanErrRows) Next() bool {
	if r.next {
		return false
	}
	r.next = true
	return true
}
func (r *scanErrRows) Scan(...any) error { return errors.New("scan fail") }
func (r *scanErrRows) Values() ([]any, error) {
	return nil, nil
}
func (r *scanErrRows) RawValues() [][]byte { return nil }
func (r *scanErrRows) Conn() *pgx.Conn     { return nil }

func newTestOrgStore() OrgUnitStore {
	return newOrgUnitMemoryStore()
}

func TestHandleSetID_TenantMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_MissingAsOfRedirects(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()

	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "as_of=") {
		t.Fatalf("location=%q", rec.Header().Get("Location"))
	}
}

func TestHandleSetID_ListScopeCodesError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, scopeTestStore{listScopeErr: errors.New("scopes boom")}, newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "scopes boom") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetID_ListScopePackagesError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, scopeTestStore{
		scopes:          []ScopeCode{{ScopeCode: "jobcatalog", OwnerModule: "jobcatalog", ShareMode: "tenant-only", IsStable: true}},
		listScopePkgErr: errors.New("pkg boom"),
	}, newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "pkg boom") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetID_ListScopePackagesSortSameScope(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, scopeTestStore{
		scopes: []ScopeCode{{ScopeCode: "jobcatalog", OwnerModule: "jobcatalog", ShareMode: "tenant-only", IsStable: true}},
		scopePackages: map[string][]ScopePackage{
			"jobcatalog": {
				{PackageID: "p2", ScopeCode: "jobcatalog", PackageCode: "PKG2", OwnerSetID: "A0001", Name: "Pkg2", Status: "active"},
				{PackageID: "p1", ScopeCode: "jobcatalog", PackageCode: "PKG1", OwnerSetID: "A0001", Name: "Pkg1", Status: "active"},
			},
		},
	}, newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_ListScopePackagesSortDifferentScope(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, scopeTestStore{
		scopes: []ScopeCode{
			{ScopeCode: "jobcatalog", OwnerModule: "jobcatalog", ShareMode: "tenant-only", IsStable: true},
			{ScopeCode: "orgunit_location", OwnerModule: "orgunit", ShareMode: "tenant-only", IsStable: true},
		},
		scopePackages: map[string][]ScopePackage{
			"jobcatalog":       {{PackageID: "p1", ScopeCode: "jobcatalog", PackageCode: "PKG1", OwnerSetID: "A0001", Name: "Pkg1", Status: "active"}},
			"orgunit_location": {{PackageID: "p2", ScopeCode: "orgunit_location", PackageCode: "PKG2", OwnerSetID: "A0001", Name: "Pkg2", Status: "active"}},
		},
	}, newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_InvalidAsOfReturns400(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=BAD", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()

	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_Get(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "SetID Governance") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleSetID_Post_CreateSetID(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_setid")
	form.Set("setid", "A0001")
	form.Set("name", "Default A")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_Post_BindSetID(t *testing.T) {
	store := newSetIDMemoryStore()

	createSetID := url.Values{}
	createSetID.Set("action", "create_setid")
	createSetID.Set("setid", "B0001")
	createSetID.Set("name", "Default B")
	reqS := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(createSetID.Encode()))
	reqS.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqS = reqS.WithContext(withTenant(reqS.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recS := httptest.NewRecorder()
	handleSetID(recS, reqS, store, newTestOrgStore())
	if recS.Code != http.StatusSeeOther {
		t.Fatalf("create setid status=%d", recS.Code)
	}

	bind := url.Values{}
	bind.Set("action", "bind_setid")
	bind.Set("org_unit_id", "10000001")
	bind.Set("setid", "B0001")
	bind.Set("effective_date", "2026-01-01")
	reqB := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(bind.Encode()))
	reqB.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqB = reqB.WithContext(withTenant(reqB.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recB := httptest.NewRecorder()
	handleSetID(recB, reqB, store, newTestOrgStore())
	if recB.Code != http.StatusSeeOther {
		t.Fatalf("bind setid status=%d", recB.Code)
	}
}

func TestHandleSetID_Post_CreateScopePackage_MissingFields(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_scope_package")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "scope_code/owner_setid/name is required") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetID_Post_CreateScopePackage_InvalidEffectiveDate(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_scope_package")
	form.Set("scope_code", "jobcatalog")
	form.Set("owner_setid", "A0001")
	form.Set("name", "Pkg")
	form.Set("effective_date", "bad")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "effective_date 无效") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetID_Post_CreateScopePackage_InvalidScope(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_scope_package")
	form.Set("scope_code", "orgunit_geo_admin")
	form.Set("owner_setid", "A0001")
	form.Set("name", "Pkg")
	form.Set("effective_date", "2026-01-01")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SCOPE_CODE_INVALID") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetID_Post_CreateScopePackage_ReservedPackageCode(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_scope_package")
	form.Set("scope_code", "jobcatalog")
	form.Set("owner_setid", "A0001")
	form.Set("name", "Pkg")
	form.Set("package_code", "DEFLT")
	form.Set("effective_date", "2026-01-01")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "PACKAGE_CODE_RESERVED") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetID_Post_CreateScopePackage_InvalidPackageCode(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_scope_package")
	form.Set("scope_code", "jobcatalog")
	form.Set("owner_setid", "A0001")
	form.Set("name", "Pkg")
	form.Set("package_code", "bad-code")
	form.Set("effective_date", "2026-01-01")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "PACKAGE_CODE_INVALID") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetID_Post_CreateScopePackage_ListScopeCodesError(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_scope_package")
	form.Set("scope_code", "jobcatalog")
	form.Set("owner_setid", "A0001")
	form.Set("name", "Pkg")
	form.Set("effective_date", "2026-01-01")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, scopeTestStore{listScopeErr: errors.New("scope fail")}, newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "scope fail") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetID_Post_CreateScopePackage_CreateError(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_scope_package")
	form.Set("scope_code", "jobcatalog")
	form.Set("owner_setid", "A0001")
	form.Set("name", "Pkg")
	form.Set("package_code", "PKG1")
	form.Set("effective_date", "2026-01-01")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, scopeTestStore{
		scopes:            []ScopeCode{{ScopeCode: "jobcatalog", OwnerModule: "jobcatalog", ShareMode: "tenant-only", IsStable: true}},
		createScopePkgErr: errors.New("boom"),
	}, newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetID_Post_CreateScopePackage_Success(t *testing.T) {
	form := url.Values{}
	form.Set("action", "create_scope_package")
	form.Set("scope_code", "jobcatalog")
	form.Set("owner_setid", "A0001")
	form.Set("name", "Pkg")
	form.Set("effective_date", "2026-01-01")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_Post_DisableScopePackage_MissingPackageID(t *testing.T) {
	form := url.Values{}
	form.Set("action", "disable_scope_package")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "package_id is required") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetID_Post_DisableScopePackage_Error(t *testing.T) {
	form := url.Values{}
	form.Set("action", "disable_scope_package")
	form.Set("package_id", "p1")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, scopeTestStore{disableScopePkgErr: errors.New("boom")}, newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetID_Post_DisableScopePackage_Success(t *testing.T) {
	store := newSetIDMemoryStore().(*setidMemoryStore)
	pkg, err := store.CreateScopePackage(context.Background(), "t1", "jobcatalog", "PKG1", "A0001", "Pkg", "2026-01-01", "r1", "i1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}

	form := url.Values{}
	form.Set("action", "disable_scope_package")
	form.Set("package_id", pkg.PackageID)

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, store, newTestOrgStore())
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_Post_ParseFormError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader("%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_MergeMsgBranches(t *testing.T) {
	newBadFormReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader("%"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	}

	recSet := httptest.NewRecorder()
	handleSetID(recSet, newBadFormReq(), partialSetIDStore{listSetErr: errors.New("boom")}, newTestOrgStore())
	if recSet.Code != http.StatusOK {
		t.Fatalf("status=%d", recSet.Code)
	}
	if body := recSet.Body.String(); !strings.Contains(body, "bad form") || !strings.Contains(body, "boom") {
		t.Fatalf("unexpected body: %q", body)
	}

	recSetEmpty := httptest.NewRecorder()
	handleSetID(recSetEmpty, newBadFormReq(), partialSetIDStore{listSetErr: emptyErr{}}, newTestOrgStore())
	if recSetEmpty.Code != http.StatusOK {
		t.Fatalf("status=%d", recSetEmpty.Code)
	}
	if body := recSetEmpty.Body.String(); !strings.Contains(body, "bad form") {
		t.Fatalf("unexpected body: %q", body)
	}

	recBind := httptest.NewRecorder()
	handleSetID(recBind, newBadFormReq(), partialSetIDStore{listBindErr: errors.New("bind boom")}, newTestOrgStore())
	if recBind.Code != http.StatusOK {
		t.Fatalf("status=%d", recBind.Code)
	}
	if body := recBind.Body.String(); !strings.Contains(body, "bad form") || !strings.Contains(body, "bind boom") {
		t.Fatalf("unexpected body: %q", body)
	}

	recOrg := httptest.NewRecorder()
	handleSetID(recOrg, newBadFormReq(), partialSetIDStore{}, errOrgUnitStore{err: errors.New("org boom")})
	if recOrg.Code != http.StatusOK {
		t.Fatalf("status=%d", recOrg.Code)
	}
	if body := recOrg.Body.String(); !strings.Contains(body, "bad form") || !strings.Contains(body, "org boom") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHandleSetID_Post_UnknownAction(t *testing.T) {
	form := url.Values{}
	form.Set("action", "nope")
	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_Post_MissingFields(t *testing.T) {
	store := newSetIDMemoryStore()

	form := url.Values{}
	form.Set("action", "create_setid")
	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, store, newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}

	form2 := url.Values{}
	form2.Set("action", "bind_setid")
	req2 := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2 = req2.WithContext(withTenant(req2.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec2 := httptest.NewRecorder()
	handleSetID(rec2, req2, store, newTestOrgStore())
	if rec2.Code != http.StatusOK {
		t.Fatalf("status=%d", rec2.Code)
	}
}

func TestHandleSetID_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_EnsureBootstrapError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, errSetIDStore{err: errors.New("boom")}, newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_Post_WriteErrors(t *testing.T) {
	createSet := url.Values{}
	createSet.Set("action", "create_setid")
	createSet.Set("setid", "A0001")
	createSet.Set("name", "A")
	reqSet := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(createSet.Encode()))
	reqSet.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqSet = reqSet.WithContext(withTenant(reqSet.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recSet := httptest.NewRecorder()
	handleSetID(recSet, reqSet, partialSetIDStore{createSetErr: errors.New("boom")}, newTestOrgStore())
	if recSet.Code != http.StatusOK {
		t.Fatalf("status=%d", recSet.Code)
	}

	bind := url.Values{}
	bind.Set("action", "bind_setid")
	bind.Set("org_unit_id", "10000001")
	bind.Set("setid", "DEFLT")
	reqBind := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(bind.Encode()))
	reqBind.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqBind = reqBind.WithContext(withTenant(reqBind.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	recBind := httptest.NewRecorder()
	handleSetID(recBind, reqBind, partialSetIDStore{bindErr: errors.New("boom")}, newTestOrgStore())
	if recBind.Code != http.StatusOK {
		t.Fatalf("status=%d", recBind.Code)
	}
}

func TestSetIDMemoryStore_Errors(t *testing.T) {
	s := newSetIDMemoryStore().(*setidMemoryStore)
	if err := s.CreateSetID(context.Background(), "t1", "", "n", "2026-01-01", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.CreateSetID(context.Background(), "t1", "SHARE", "n", "2026-01-01", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.CreateSetID(context.Background(), "t1", "A0001", "n", "2026-01-01", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.CreateSetID(context.Background(), "t1", "A0001", "n", "2026-01-01", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.BindSetID(context.Background(), "t1", "", "2026-01-01", "A0001", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.BindSetID(context.Background(), "t1", "10000001", "2026-01-01", "", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.BindSetID(context.Background(), "t1", "10000001", "2026-01-01", "NOPE", "", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.BindSetID(context.Background(), "t1", "10000001", "2026-01-01", "A0001", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestSetIDMemoryStore_ListSortsWithMultipleItems(t *testing.T) {
	s := newSetIDMemoryStore().(*setidMemoryStore)
	if err := s.EnsureBootstrap(context.Background(), "t1", "i1"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.CreateSetID(context.Background(), "t1", "B0001", "B", "2026-01-01", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.CreateSetID(context.Background(), "t1", "A0001", "A", "2026-01-01", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.BindSetID(context.Background(), "t1", "org-b", "2026-01-01", "B0001", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := s.BindSetID(context.Background(), "t1", "org-a", "2026-01-01", "A0001", "", ""); err != nil {
		t.Fatalf("err=%v", err)
	}

	setids, err := s.ListSetIDs(context.Background(), "t1")
	if err != nil || len(setids) < 2 {
		t.Fatalf("len=%d err=%v", len(setids), err)
	}
	if setids[0].SetID != "A0001" {
		t.Fatalf("unexpected first setid=%q", setids[0].SetID)
	}

	bindings, err := s.ListSetIDBindings(context.Background(), "t1", "2026-01-01")
	if err != nil || len(bindings) < 2 {
		t.Fatalf("len=%d err=%v", len(bindings), err)
	}
	if bindings[0].OrgUnitID != "org-a" {
		t.Fatalf("unexpected first binding org=%q", bindings[0].OrgUnitID)
	}
}

func TestSetIDMemoryStore_ScopePackageLifecycle(t *testing.T) {
	store := newSetIDMemoryStore().(*setidMemoryStore)
	pkg, err := store.CreateScopePackage(context.Background(), "t1", "jobcatalog", "PKG1", "A0001", "Pkg", "2026-01-01", "r1", "i1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if pkg.EffectiveDate != "2026-01-01" {
		t.Fatalf("effective_date=%q", pkg.EffectiveDate)
	}
	if pkg.UpdatedAt == "" {
		t.Fatal("expected updated_at")
	}

	pkgs, err := store.ListScopePackages(context.Background(), "t1", "jobcatalog")
	if err != nil || len(pkgs) != 1 {
		t.Fatalf("len=%d err=%v", len(pkgs), err)
	}

	disabled, err := store.DisableScopePackage(context.Background(), "t1", pkg.PackageID, "r2", "i1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if disabled.Status != "disabled" {
		t.Fatalf("status=%q", disabled.Status)
	}
	if disabled.UpdatedAt == "" {
		t.Fatal("expected updated_at")
	}

	if _, err := store.DisableScopePackage(context.Background(), "t1", "missing", "r3", "i1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetIDMemoryStore_CreateGlobalSetID(t *testing.T) {
	s := newSetIDMemoryStore().(*setidMemoryStore)
	if err := s.CreateGlobalSetID(context.Background(), "", "", "", "saas"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.CreateGlobalSetID(context.Background(), "Shared", "", "", "tenant"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.CreateGlobalSetID(context.Background(), "Shared", "", "", "saas"); err != nil {
		t.Fatalf("err=%v", err)
	}
	if s.globalSetIDName != "Shared" {
		t.Fatalf("name=%q", s.globalSetIDName)
	}
}

func TestRenderSetIDPage_SkipsDisabledOptions(t *testing.T) {
	html := renderSetIDPage(
		[]SetID{{SetID: "SHARE", Name: "Shared", Status: "active", IsShared: true}, {SetID: "A0001", Name: "A", Status: "disabled"}},
		[]SetIDBindingRow{{OrgUnitID: "10000001", SetID: "SHARE", ValidFrom: "2026-01-01"}},
		[]OrgUnitNode{{ID: "10000001", Name: "BU 1", IsBusinessUnit: true}, {ID: "10000002", Name: "BU 0", IsBusinessUnit: true}},
		nil,
		nil,
		Tenant{Name: "T"},
		"2026-01-07",
		"",
		"en",
		"",
	)
	if strings.Contains(html, "option value=\"SHARE\"") {
		t.Fatalf("unexpected html: %q", html)
	}
	if strings.Contains(html, "option value=\"A0001\"") {
		t.Fatalf("unexpected html: %q", html)
	}
	if !strings.Contains(html, "Shared/Read-only (共享/只读)") {
		t.Fatalf("unexpected html: %q", html)
	}
}

func TestRenderSetIDPage_ScopePackagesNoActiveSetIDs(t *testing.T) {
	html := renderSetIDPage(
		[]SetID{{SetID: "SHARE", Name: "Shared", Status: "active", IsShared: true}, {SetID: "A0001", Name: "A", Status: "disabled"}},
		nil,
		nil,
		[]ScopeCode{{ScopeCode: "jobcatalog", OwnerModule: "jobcatalog", ShareMode: "tenant-only", IsStable: true}},
		nil,
		Tenant{Name: "T"},
		"2026-01-07",
		"",
		"en",
		"",
	)
	if !strings.Contains(html, "(no active setids)") {
		t.Fatalf("unexpected html: %q", html)
	}
}

func TestRenderSetIDPage_ScopePackages(t *testing.T) {
	html := renderSetIDPage(
		[]SetID{{SetID: "A0001", Name: "A", Status: "active"}},
		nil,
		nil,
		[]ScopeCode{{ScopeCode: "jobcatalog", OwnerModule: "jobcatalog", ShareMode: "tenant-only", IsStable: true}},
		[]ScopePackage{
			{PackageID: "p1", ScopeCode: "jobcatalog", PackageCode: "PKG1", OwnerSetID: "A0001", Name: "Pkg1", Status: "active", EffectiveDate: "2026-01-01", UpdatedAt: "2026-01-02T00:00:00Z"},
			{PackageID: "p2", ScopeCode: "jobcatalog", PackageCode: "DEFLT", OwnerSetID: "", Name: "Default", Status: "active"},
			{PackageID: "p3", ScopeCode: "jobcatalog", PackageCode: "PKG2", OwnerSetID: "A0001", Name: "Pkg2", Status: "disabled"},
		},
		Tenant{Name: "T"},
		"2026-01-07",
		"",
		"en",
		"",
	)
	if !strings.Contains(html, "Create Package") {
		t.Fatalf("unexpected html: %q", html)
	}
	if !strings.Contains(html, "option value=\"A0001\"") {
		t.Fatalf("unexpected html: %q", html)
	}
	if !strings.Contains(html, "(protected)") {
		t.Fatalf("unexpected html: %q", html)
	}
	if !strings.Contains(html, "(disabled)") {
		t.Fatalf("unexpected html: %q", html)
	}
	if !strings.Contains(html, "Disable</button>") {
		t.Fatalf("unexpected html: %q", html)
	}
}

func TestRenderSetIDPage_NoBusinessUnits(t *testing.T) {
	html := renderSetIDPage(
		[]SetID{{SetID: "DEFLT", Name: "Default", Status: "active"}},
		nil,
		[]OrgUnitNode{{ID: "10000001", Name: "Org 1", IsBusinessUnit: false}},
		nil,
		nil,
		Tenant{Name: "T"},
		"2026-01-07",
		"",
		"en",
		"",
	)
	if !strings.Contains(html, "(no business units)") {
		t.Fatalf("unexpected html: %q", html)
	}
}

func TestRenderSetIDPage_SelectedSetID(t *testing.T) {
	html := renderSetIDPage(
		[]SetID{{SetID: "A0001", Name: "A", Status: "active"}},
		nil,
		nil,
		nil,
		nil,
		Tenant{Name: "T"},
		"2026-01-07",
		"A0001",
		"en",
		"",
	)
	if !strings.Contains(html, "/orgunit/setids/A0001/scope-subscriptions") {
		t.Fatalf("unexpected html: %q", html)
	}
}

func TestHandleSetID_Post_DefaultAction(t *testing.T) {
	form := url.Values{}
	form.Set("setid", "A0001")
	form.Set("name", "Default A")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleSetID_OrgStoreMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/setid?as_of=2026-01-01", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "orgunit store missing") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleSetID_BindInvalidEffectiveDate(t *testing.T) {
	form := url.Values{}
	form.Set("action", "bind_setid")
	form.Set("org_unit_id", "10000001")
	form.Set("setid", "DEFLT")
	form.Set("effective_date", "bad")

	req := httptest.NewRequest(http.MethodPost, "/org/setid?as_of=2026-01-01", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
	rec := httptest.NewRecorder()
	handleSetID(rec, req, newSetIDMemoryStore(), newTestOrgStore())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "effective_date 无效") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestSetIDPGStore_WithTx_Errors(t *testing.T) {
	s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin fail")
	})}
	if err := s.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	tx := &stubTx{execErr: errors.New("set_config fail"), execErrAt: 1}
	s2 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
	if err := s2.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	tx2 := &stubTx{commitErr: errors.New("commit fail")}
	s3 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx2, nil })}
	if err := s3.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetIDPGStore_EnsureBootstrap_GlobalShare(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return &stubTx{row: &stubRow{vals: []any{"gt1"}}}, nil
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("global begin error", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return nil, errors.New("begin fail")
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("global tenant id error", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return &stubTx{rowErr: errors.New("row fail")}, nil
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("global set_config current_tenant error", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return &stubTx{
				row:       &stubRow{vals: []any{"gt1"}},
				execErr:   errors.New("exec fail"),
				execErrAt: 1,
			}, nil
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("global set_config actor_scope error", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return &stubTx{
				row:       &stubRow{vals: []any{"gt1"}},
				execErr:   errors.New("exec fail"),
				execErrAt: 2,
			}, nil
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("global set_config allow_share_read error", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return &stubTx{
				row:       &stubRow{vals: []any{"gt1"}},
				execErr:   errors.New("exec fail"),
				execErrAt: 3,
			}, nil
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("global submit error", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return &stubTx{
				row:       &stubRow{vals: []any{"gt1"}},
				execErr:   errors.New("exec fail"),
				execErrAt: 4,
			}, nil
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("global commit error", func(t *testing.T) {
		var calls int
		store := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return &stubTx{}, nil
			}
			return &stubTx{
				row:       &stubRow{vals: []any{"gt1"}},
				commitErr: errors.New("commit fail"),
			}, nil
		})}
		if err := store.EnsureBootstrap(context.Background(), "t1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestSetIDPGStore_ListSetIDs(t *testing.T) {
	txTenant := &stubTx{
		rows: &tableRows{rows: [][]any{
			{"A0001", "A", "active"},
		}},
	}
	txGlobal := &stubTx{
		row: &stubRow{vals: []any{"gt1"}},
		rows: &tableRows{rows: [][]any{
			{"SHARE", "Shared", "active"},
		}},
	}
	var calls int
	s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		calls++
		if calls == 1 {
			return txTenant, nil
		}
		return txGlobal, nil
	})}

	if got, err := s.ListSetIDs(context.Background(), "t1"); err != nil || len(got) != 2 {
		t.Fatalf("len=%d err=%v", len(got), err)
	}

	txQueryErr := &stubTx{queryErr: errors.New("query fail")}
	sQueryErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txQueryErr, nil })}
	if _, err := sQueryErr.ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	txGlobalQueryErr := &stubTx{
		row:      &stubRow{vals: []any{"gt1"}},
		queryErr: errors.New("global query fail"),
	}
	calls = 0
	sGlobalQueryErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		calls++
		if calls == 1 {
			return &stubTx{rows: &tableRows{rows: [][]any{{"A0001", "A", "active"}}}}, nil
		}
		return txGlobalQueryErr, nil
	})}
	if _, err := sGlobalQueryErr.ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	txErrScan := &stubTx{rows: &scanErrRows{}}
	sErrScan := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txErrScan, nil })}
	if _, err := sErrScan.ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	txErrRows := &stubTx{rows: &tableRows{rows: [][]any{}, err: errors.New("rows err")}}
	sErrRows := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txErrRows, nil })}
	if _, err := sErrRows.ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	txCommitErr := &stubTx{commitErr: errors.New("commit fail"), rows: &tableRows{rows: [][]any{}}}
	sCommitErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txCommitErr, nil })}
	if _, err := sCommitErr.ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetIDPGStore_ListSetIDs_GlobalErrors(t *testing.T) {
	tenantTx := func() *stubTx {
		return &stubTx{rows: &tableRows{rows: [][]any{{"A0001", "A", "active"}}}}
	}
	makeStore := func(globalTx pgx.Tx, globalErr error) *setidPGStore {
		var calls int
		return &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			calls++
			if calls == 1 {
				return tenantTx(), nil
			}
			if globalErr != nil {
				return nil, globalErr
			}
			return globalTx, nil
		})}
	}

	if _, err := makeStore(nil, errors.New("begin fail")).ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	if _, err := makeStore(&stubTx{rowErr: errors.New("row fail")}, nil).ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	if _, err := makeStore(&stubTx{
		row:       &stubRow{vals: []any{"gt1"}},
		execErr:   errors.New("exec fail"),
		execErrAt: 1,
	}, nil).ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	if _, err := makeStore(&stubTx{
		row:       &stubRow{vals: []any{"gt1"}},
		execErr:   errors.New("exec fail"),
		execErrAt: 2,
	}, nil).ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	if _, err := makeStore(&stubTx{
		row:  &stubRow{vals: []any{"gt1"}},
		rows: &scanErrRows{},
	}, nil).ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	if _, err := makeStore(&stubTx{
		row:  &stubRow{vals: []any{"gt1"}},
		rows: &tableRows{rows: [][]any{}, err: errors.New("rows err")},
	}, nil).ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	if _, err := makeStore(&stubTx{
		row:       &stubRow{vals: []any{"gt1"}},
		rows:      &tableRows{rows: [][]any{}},
		commitErr: errors.New("commit fail"),
	}, nil).ListSetIDs(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetIDPGStore_ListSetIDBindings(t *testing.T) {
	tx := &stubTx{
		rows: &tableRows{rows: [][]any{
			{"10000001", "SHARE", "2026-01-01", ""},
		}},
	}
	s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}

	rows, err := s.ListSetIDBindings(context.Background(), "t1", "2026-01-01")
	if err != nil || len(rows) != 1 {
		t.Fatalf("len=%d err=%v", len(rows), err)
	}
	if rows[0].OrgUnitID != "10000001" {
		t.Fatalf("unexpected org=%q", rows[0].OrgUnitID)
	}

	txQueryErr := &stubTx{queryErr: errors.New("query fail")}
	sQueryErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txQueryErr, nil })}
	if _, err := sQueryErr.ListSetIDBindings(context.Background(), "t1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	txErrScan := &stubTx{rows: &scanErrRows{}}
	sErrScan := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txErrScan, nil })}
	if _, err := sErrScan.ListSetIDBindings(context.Background(), "t1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	txErrRows := &stubTx{rows: &tableRows{rows: [][]any{}, err: errors.New("rows err")}}
	sErrRows := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txErrRows, nil })}
	if _, err := sErrRows.ListSetIDBindings(context.Background(), "t1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}

	txCommitErr := &stubTx{commitErr: errors.New("commit fail"), rows: &tableRows{rows: [][]any{}}}
	sCommitErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txCommitErr, nil })}
	if _, err := sCommitErr.ListSetIDBindings(context.Background(), "t1", "2026-01-01"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetIDPGStore_CreateSetID_Errors(t *testing.T) {
	tx1 := &stubTx{execErr: errors.New("exec fail"), execErrAt: 1}
	s1 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx1, nil })}
	if err := s1.CreateSetID(context.Background(), "t1", "A0001", "A", "2026-01-01", "r1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	tx2 := &stubTx{execErr: errors.New("exec fail"), execErrAt: 2}
	s2 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx2, nil })}
	if err := s2.CreateSetID(context.Background(), "t1", "A0001", "A", "2026-01-01", "r1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	tx3 := &stubTx{}
	s3 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3, nil })}
	withRandReader(t, randErrReader{}, func() {
		if err := s3.CreateSetID(context.Background(), "t1", "A0001", "A", "2026-01-01", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestSetIDPGStore_BindSetID_Errors(t *testing.T) {
	tx1 := &stubTx{}
	s1 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx1, nil })}
	if err := s1.BindSetID(context.Background(), "t1", "bad", "2026-01-01", "SHARE", "r1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	tx2 := &stubTx{execErr: errors.New("exec fail"), execErrAt: 2}
	s2 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx2, nil })}
	if err := s2.BindSetID(context.Background(), "t1", "10000001", "2026-01-01", "SHARE", "r1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	tx3 := &stubTx{execErr: errors.New("exec fail"), execErrAt: 3}
	s3 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx3, nil })}
	if err := s3.BindSetID(context.Background(), "t1", "10000001", "2026-01-01", "SHARE", "r1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	tx4 := &stubTx{}
	s4 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx4, nil })}
	withRandReader(t, randErrReader{}, func() {
		if err := s4.BindSetID(context.Background(), "t1", "10000001", "2026-01-01", "SHARE", "r1", "p1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestSetIDPGStore_EnsureGlobalShareSetID(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin fail")
		})}
		if err := s.ensureGlobalShareSetID(context.Background(), "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("global tenant id error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("row fail")}
		s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if err := s.ensureGlobalShareSetID(context.Background(), "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config current tenant error", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{"gt1"}}, execErr: errors.New("exec fail"), execErrAt: 1}
		s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if err := s.ensureGlobalShareSetID(context.Background(), "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config actor scope error", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{"gt1"}}, execErr: errors.New("exec fail"), execErrAt: 2}
		s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if err := s.ensureGlobalShareSetID(context.Background(), "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config allow share read error", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{"gt1"}}, execErr: errors.New("exec fail"), execErrAt: 3}
		s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if err := s.ensureGlobalShareSetID(context.Background(), "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("event id error", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{"gt1"}}}
		s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		withRandReader(t, randErrReader{}, func() {
			if err := s.ensureGlobalShareSetID(context.Background(), "p1"); err == nil {
				t.Fatal("expected error")
			}
		})
	})

	t.Run("submit error", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{"gt1"}}, execErr: errors.New("exec fail"), execErrAt: 4}
		s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if err := s.ensureGlobalShareSetID(context.Background(), "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{"gt1"}}, commitErr: errors.New("commit fail")}
		s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if err := s.ensureGlobalShareSetID(context.Background(), "p1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		tx := &stubTx{row: &stubRow{vals: []any{"gt1"}}}
		s := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if err := s.ensureGlobalShareSetID(context.Background(), "p1"); err != nil {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestSetIDPGStore_CreateGlobalSetID(t *testing.T) {
	sBeginErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
		return nil, errors.New("begin fail")
	})}
	if err := sBeginErr.CreateGlobalSetID(context.Background(), "Shared", "r1", "p1", "saas"); err == nil {
		t.Fatal("expected error")
	}

	txRowErr := &stubTx{rowErr: errors.New("row fail")}
	sRowErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txRowErr, nil })}
	if err := sRowErr.CreateGlobalSetID(context.Background(), "Shared", "r1", "p1", "saas"); err == nil {
		t.Fatal("expected error")
	}

	txExecErr := &stubTx{row: &stubRow{vals: []any{"gt1"}}, execErr: errors.New("exec fail"), execErrAt: 1}
	sExecErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txExecErr, nil })}
	if err := sExecErr.CreateGlobalSetID(context.Background(), "Shared", "r1", "p1", "saas"); err == nil {
		t.Fatal("expected error")
	}

	txExecErr2 := &stubTx{row: &stubRow{vals: []any{"gt1"}}, execErr: errors.New("exec fail"), execErrAt: 2}
	sExecErr2 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txExecErr2, nil })}
	if err := sExecErr2.CreateGlobalSetID(context.Background(), "Shared", "r1", "p1", "saas"); err == nil {
		t.Fatal("expected error")
	}

	txExecErr3 := &stubTx{row: &stubRow{vals: []any{"gt1"}}, execErr: errors.New("exec fail"), execErrAt: 3}
	sExecErr3 := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txExecErr3, nil })}
	if err := sExecErr3.CreateGlobalSetID(context.Background(), "Shared", "r1", "p1", "saas"); err == nil {
		t.Fatal("expected error")
	}

	txEventErr := &stubTx{row: &stubRow{vals: []any{"gt1"}}}
	sEventErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txEventErr, nil })}
	withRandReader(t, randErrReader{}, func() {
		if err := sEventErr.CreateGlobalSetID(context.Background(), "Shared", "r1", "p1", "saas"); err == nil {
			t.Fatal("expected error")
		}
	})

	txCommitErr := &stubTx{row: &stubRow{vals: []any{"gt1"}}, commitErr: errors.New("commit fail")}
	sCommitErr := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txCommitErr, nil })}
	if err := sCommitErr.CreateGlobalSetID(context.Background(), "Shared", "r1", "p1", "saas"); err == nil {
		t.Fatal("expected error")
	}

	txOK := &stubTx{row: &stubRow{vals: []any{"gt1"}}}
	sOK := &setidPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txOK, nil })}
	if err := sOK.CreateGlobalSetID(context.Background(), "Shared", "r1", "p1", "saas"); err != nil {
		t.Fatalf("err=%v", err)
	}
}
