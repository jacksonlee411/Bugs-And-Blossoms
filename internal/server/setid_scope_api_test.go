package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type scopeAPIStore struct {
	listScopePackagesFn        func(context.Context, string, string) ([]ScopePackage, error)
	listOwnedScopePackagesFn   func(context.Context, string, string, string) ([]OwnedScopePackage, error)
	resolveSetIDFn             func(context.Context, string, string, string) (string, error)
	createScopePackageFn       func(context.Context, string, string, string, string, string, string, string, string) (ScopePackage, error)
	disableScopePackageFn      func(context.Context, string, string, string, string, string) (ScopePackage, error)
	createScopeSubscriptionFn  func(context.Context, string, string, string, string, string, string, string, string) (ScopeSubscription, error)
	getScopeSubscriptionFn     func(context.Context, string, string, string, string) (ScopeSubscription, error)
	createGlobalScopePackageFn func(context.Context, string, string, string, string, string, string, string) (ScopePackage, error)
	listGlobalScopePackagesFn  func(context.Context, string) ([]ScopePackage, error)
}

func (s scopeAPIStore) EnsureBootstrap(context.Context, string, string) error { return nil }
func (s scopeAPIStore) ListSetIDs(context.Context, string) ([]SetID, error)   { return nil, nil }
func (s scopeAPIStore) ListGlobalSetIDs(context.Context) ([]SetID, error)     { return nil, nil }
func (s scopeAPIStore) CreateSetID(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s scopeAPIStore) ListSetIDBindings(context.Context, string, string) ([]SetIDBindingRow, error) {
	return nil, nil
}
func (s scopeAPIStore) BindSetID(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (s scopeAPIStore) ResolveSetID(ctx context.Context, tenantID string, orgUnitID string, asOfDate string) (string, error) {
	if s.resolveSetIDFn != nil {
		return s.resolveSetIDFn(ctx, tenantID, orgUnitID, asOfDate)
	}
	return "A0001", nil
}
func (s scopeAPIStore) CreateGlobalSetID(context.Context, string, string, string, string) error {
	return nil
}
func (s scopeAPIStore) ListScopeCodes(context.Context, string) ([]ScopeCode, error) {
	return nil, nil
}
func (s scopeAPIStore) CreateScopePackage(ctx context.Context, tenantID string, scopeCode string, packageCode string, ownerSetID string, name string, effectiveDate string, requestID string, initiatorID string) (ScopePackage, error) {
	if s.createScopePackageFn == nil {
		return ScopePackage{}, nil
	}
	return s.createScopePackageFn(ctx, tenantID, scopeCode, packageCode, ownerSetID, name, effectiveDate, requestID, initiatorID)
}
func (s scopeAPIStore) DisableScopePackage(ctx context.Context, tenantID string, packageID string, effectiveDate string, requestID string, initiatorID string) (ScopePackage, error) {
	if s.disableScopePackageFn == nil {
		return ScopePackage{}, nil
	}
	return s.disableScopePackageFn(ctx, tenantID, packageID, effectiveDate, requestID, initiatorID)
}
func (s scopeAPIStore) ListScopePackages(ctx context.Context, tenantID string, scopeCode string) ([]ScopePackage, error) {
	if s.listScopePackagesFn == nil {
		return nil, nil
	}
	return s.listScopePackagesFn(ctx, tenantID, scopeCode)
}
func (s scopeAPIStore) ListOwnedScopePackages(ctx context.Context, tenantID string, scopeCode string, asOfDate string) ([]OwnedScopePackage, error) {
	if s.listOwnedScopePackagesFn == nil {
		return nil, nil
	}
	return s.listOwnedScopePackagesFn(ctx, tenantID, scopeCode, asOfDate)
}
func (s scopeAPIStore) CreateScopeSubscription(ctx context.Context, tenantID string, setID string, scopeCode string, packageID string, packageOwner string, effectiveDate string, requestID string, initiatorID string) (ScopeSubscription, error) {
	if s.createScopeSubscriptionFn == nil {
		return ScopeSubscription{}, nil
	}
	return s.createScopeSubscriptionFn(ctx, tenantID, setID, scopeCode, packageID, packageOwner, effectiveDate, requestID, initiatorID)
}
func (s scopeAPIStore) GetScopeSubscription(ctx context.Context, tenantID string, setID string, scopeCode string, asOfDate string) (ScopeSubscription, error) {
	if s.getScopeSubscriptionFn == nil {
		return ScopeSubscription{}, nil
	}
	return s.getScopeSubscriptionFn(ctx, tenantID, setID, scopeCode, asOfDate)
}
func (s scopeAPIStore) CreateGlobalScopePackage(ctx context.Context, scopeCode string, packageCode string, name string, effectiveDate string, requestID string, initiatorID string, actorScope string) (ScopePackage, error) {
	if s.createGlobalScopePackageFn == nil {
		return ScopePackage{}, nil
	}
	return s.createGlobalScopePackageFn(ctx, scopeCode, packageCode, name, effectiveDate, requestID, initiatorID, actorScope)
}
func (s scopeAPIStore) ListGlobalScopePackages(ctx context.Context, scopeCode string) ([]ScopePackage, error) {
	if s.listGlobalScopePackagesFn == nil {
		return nil, nil
	}
	return s.listGlobalScopePackagesFn(ctx, scopeCode)
}

func TestHandleScopePackagesAPI_Get(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/scope-packages?scope_code=jobcatalog", nil)
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("empty rows", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/scope-packages?scope_code=jobcatalog", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{
			listScopePackagesFn: func(context.Context, string, string) ([]ScopePackage, error) {
				return nil, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if strings.TrimSpace(rec.Body.String()) == "" {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("actor scope optional", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/scope-packages?scope_code=jobcatalog", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{
			listScopePackagesFn: func(context.Context, string, string) ([]ScopePackage, error) {
				return []ScopePackage{}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("scope_code required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/scope-packages", nil)
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/scope-packages?scope_code=jobcatalog", nil)
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{
			listScopePackagesFn: func(context.Context, string, string) ([]ScopePackage, error) {
				return nil, errors.New("SCOPE_CODE_INVALID")
			},
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "SCOPE_CODE_INVALID") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/scope-packages?scope_code=jobcatalog", nil)
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{
			listScopePackagesFn: func(context.Context, string, string) ([]ScopePackage, error) {
				return []ScopePackage{{PackageID: "p1", ScopeCode: "jobcatalog", PackageCode: "PKG1", Status: "active"}}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		var got []ScopePackage
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decode err=%v", err)
		}
		if len(got) != 1 || got[0].PackageID != "p1" {
			t.Fatalf("unexpected payload: %+v", got)
		}
	})
}

func TestHandleOwnedScopePackagesAPI_Get(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/owned-scope-packages?scope_code=jobcatalog", nil)
		rec := httptest.NewRecorder()
		handleOwnedScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/owned-scope-packages?scope_code=jobcatalog", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOwnedScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("scope_code required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/owned-scope-packages", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
		rec := httptest.NewRecorder()
		handleOwnedScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/owned-scope-packages?scope_code=jobcatalog&as_of=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
		rec := httptest.NewRecorder()
		handleOwnedScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("no principal returns empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/owned-scope-packages?scope_code=jobcatalog&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleOwnedScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("empty role returns empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/owned-scope-packages?scope_code=jobcatalog&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: " ", Status: "active"}))
		rec := httptest.NewRecorder()
		handleOwnedScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("not admin returns empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/owned-scope-packages?scope_code=jobcatalog&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-viewer", Status: "active"}))
		rec := httptest.NewRecorder()
		handleOwnedScopePackagesAPI(rec, req, scopeAPIStore{
			listOwnedScopePackagesFn: func(context.Context, string, string, string) ([]OwnedScopePackage, error) {
				return []OwnedScopePackage{{PackageID: "p1"}}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("store error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/owned-scope-packages?scope_code=jobcatalog&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
		rec := httptest.NewRecorder()
		handleOwnedScopePackagesAPI(rec, req, scopeAPIStore{
			listOwnedScopePackagesFn: func(context.Context, string, string, string) ([]OwnedScopePackage, error) {
				return nil, errors.New("boom")
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("as_of required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/owned-scope-packages?scope_code=jobcatalog", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
		rec := httptest.NewRecorder()
		handleOwnedScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("returns rows", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/owned-scope-packages?scope_code=jobcatalog&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
		rec := httptest.NewRecorder()
		handleOwnedScopePackagesAPI(rec, req, scopeAPIStore{
			listOwnedScopePackagesFn: func(context.Context, string, string, string) ([]OwnedScopePackage, error) {
				return []OwnedScopePackage{
					{
						PackageID:     "p1",
						ScopeCode:     "jobcatalog",
						PackageCode:   "PKG1",
						OwnerSetID:    "A0001",
						Name:          "Pkg",
						Status:        "active",
						EffectiveDate: "2026-01-01",
					},
				}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "PKG1") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("nil rows normalized to empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/owned-scope-packages?scope"+"_code=jobcatalog&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", RoleSlug: "tenant-admin", Status: "active"}))
		rec := httptest.NewRecorder()
		handleOwnedScopePackagesAPI(rec, req, scopeAPIStore{
			listOwnedScopePackagesFn: func(context.Context, string, string, string) ([]OwnedScopePackage, error) {
				return nil, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})
}

func TestHandleScopePackagesAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/org/api/scope-packages", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handleScopePackagesAPI(rec, req, scopeAPIStore{})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleScopePackagesAPI_Post(t *testing.T) {
	t.Run("bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid request", func(t *testing.T) {
		body := bytes.NewBufferString(`{"scope_code":"","name":"","request_id":""}`)
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages", body)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid effective date", func(t *testing.T) {
		body := bytes.NewBufferString(`{"scope_code":"jobcatalog","owner_setid":"A0001","business_unit_id":"10000001","name":"Pkg","effective_date":"bad","request_id":"r1"}`)
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages", body)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("effective date required", func(t *testing.T) {
		body := bytes.NewBufferString(`{"scope_code":"jobcatalog","owner_setid":"A0001","business_unit_id":"10000001","name":"Pkg","effective_date":"","request_id":"r1"}`)
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages", body)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("reserved code", func(t *testing.T) {
		body := bytes.NewBufferString(`{"scope_code":"jobcatalog","package_code":"DEFLT","owner_setid":"A0001","business_unit_id":"10000001","name":"Pkg","effective_date":"2026-01-01","request_id":"r1"}`)
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages", body)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid code", func(t *testing.T) {
		body := bytes.NewBufferString(`{"scope_code":"jobcatalog","package_code":"bad-code","owner_setid":"A0001","business_unit_id":"10000001","name":"Pkg","effective_date":"2026-01-01","request_id":"r1"}`)
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages", body)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("create error", func(t *testing.T) {
		body := bytes.NewBufferString(`{"scope_code":"jobcatalog","package_code":"PKG1","owner_setid":"A0001","business_unit_id":"10000001","name":"Pkg","effective_date":"2026-01-01","request_id":"r1"}`)
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages", body)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{
			createScopePackageFn: func(context.Context, string, string, string, string, string, string, string, string) (ScopePackage, error) {
				return ScopePackage{}, errors.New("PACKAGE_CODE_DUPLICATE")
			},
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("owner context forbidden", func(t *testing.T) {
		body := bytes.NewBufferString(`{"scope_code":"jobcatalog","package_code":"PKG1","owner_setid":"B0001","business_unit_id":"10000001","name":"Pkg","effective_date":"2026-01-01","request_id":"r1"}`)
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages", body)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), scopeReasonOwnerContextForbidden) {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		body := bytes.NewBufferString(`{"scope_code":"jobcatalog","package_code":"","owner_setid":"A0001","business_unit_id":"10000001","name":"Pkg","effective_date":"2026-01-01","request_id":"r1"}`)
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages", body)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackagesAPI(rec, req, scopeAPIStore{
			createScopePackageFn: func(context.Context, string, string, string, string, string, string, string, string) (ScopePackage, error) {
				return ScopePackage{PackageID: "p1", ScopeCode: "jobcatalog", PackageCode: "PKG1", OwnerSetID: "A0001", Status: "active"}, nil
			},
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "p1") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})
}

func TestHandleScopePackageDisableAPI(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages/p1/disable", strings.NewReader(`{"owner_setid":"A0001","business_unit_id":"10000001","effective_date":"2026-01-01","request_id":"r1"}`))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/scope-packages/p1/disable", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages/p1/disable", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing owner context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages/p1/disable", strings.NewReader(`{"effective_date":"2026-01-01","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing request id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages/p1/disable", strings.NewReader(`{"owner_setid":"A0001","business_unit_id":"10000001","effective_date":"2026-01-01","request_id":""}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("missing effective date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages/p1/disable", strings.NewReader(`{"owner_setid":"A0001","business_unit_id":"10000001","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid effective date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages/p1/disable", strings.NewReader(`{"owner_setid":"A0001","business_unit_id":"10000001","effective_date":"bad","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages/p1/disable", strings.NewReader(`{"owner_setid":"A0001","business_unit_id":"10000001","effective_date":"2026-01-01","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{
			disableScopePackageFn: func(context.Context, string, string, string, string, string) (ScopePackage, error) {
				return ScopePackage{}, errors.New("PACKAGE_NOT_FOUND")
			},
		})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("owner context forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages/p1/disable", strings.NewReader(`{"owner_setid":"B0001","business_unit_id":"10000001","effective_date":"2026-01-01","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), scopeReasonOwnerContextForbidden) {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages/p1/disable", strings.NewReader(`{"owner_setid":"A0001","business_unit_id":"10000001","effective_date":"2026-01-01","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopePackageDisableAPI(rec, req, scopeAPIStore{
			disableScopePackageFn: func(context.Context, string, string, string, string, string) (ScopePackage, error) {
				return ScopePackage{PackageID: "p1", Status: "disabled"}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "disabled") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})
}

func TestHandleScopeSubscriptionsAPI(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/scope-subscriptions?setid=S2601&scope_code=jobcatalog&as_of=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get missing params", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/scope-subscriptions", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get invalid date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/scope-subscriptions?setid=S2601&scope_code=jobcatalog&as_of=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get store error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/scope-subscriptions?setid=S2601&scope_code=jobcatalog&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{
			getScopeSubscriptionFn: func(context.Context, string, string, string, string) (ScopeSubscription, error) {
				return ScopeSubscription{}, errors.New("SCOPE_SUBSCRIPTION_MISSING")
			},
		})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/scope-subscriptions?setid=S2601&scope_code=jobcatalog&as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{
			getScopeSubscriptionFn: func(context.Context, string, string, string, string) (ScopeSubscription, error) {
				return ScopeSubscription{SetID: "S2601", ScopeCode: "jobcatalog", PackageID: "p1", PackageOwner: "tenant", EffectiveDate: "2026-01-01", EndDate: ""}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "S2601") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("get as_of required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/scope-subscriptions?setid=S2601&scope_code=jobcatalog", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-subscriptions", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post missing fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-subscriptions", strings.NewReader(`{"setid":""}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post invalid date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-subscriptions", strings.NewReader(`{"setid":"A0001","scope_code":"jobcatalog","package_id":"p1","package_owner":"tenant","business_unit_id":"10000001","effective_date":"bad","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post owner invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-subscriptions", strings.NewReader(`{"setid":"A0001","scope_code":"jobcatalog","package_id":"p1","package_owner":"nope","business_unit_id":"10000001","effective_date":"2026-01-01","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post context forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-subscriptions", strings.NewReader(`{"setid":"B0001","scope_code":"jobcatalog","package_id":"p1","package_owner":"tenant","business_unit_id":"10000001","effective_date":"2026-01-01","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), scopeReasonOwnerContextForbidden) {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("post store error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-subscriptions", strings.NewReader(`{"setid":"A0001","scope_code":"jobcatalog","package_id":"p1","package_owner":"tenant","business_unit_id":"10000001","effective_date":"2026-01-01","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{
			createScopeSubscriptionFn: func(context.Context, string, string, string, string, string, string, string, string) (ScopeSubscription, error) {
				return ScopeSubscription{}, errors.New("SUBSCRIPTION_OVERLAP")
			},
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-subscriptions", strings.NewReader(`{"setid":"A0001","scope_code":"jobcatalog","package_id":"p1","package_owner":"tenant","business_unit_id":"10000001","effective_date":"2026-01-01","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{
			createScopeSubscriptionFn: func(context.Context, string, string, string, string, string, string, string, string) (ScopeSubscription, error) {
				return ScopeSubscription{SetID: "A0001", ScopeCode: "jobcatalog", PackageID: "p1", PackageOwner: "tenant", EffectiveDate: "2026-01-01"}, nil
			},
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandleScopeSubscriptionsAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/org/api/scope-subscriptions", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handleScopeSubscriptionsAPI(rec, req, scopeAPIStore{})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleGlobalScopePackagesAPI(t *testing.T) {
	t.Run("get actor scope required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/global-scope-packages?scope_code=jobcatalog", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get scope_code required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/global-scope-packages", nil)
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get store error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/global-scope-packages?scope_code=jobcatalog", nil)
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{
			listGlobalScopePackagesFn: func(context.Context, string) ([]ScopePackage, error) {
				return nil, errors.New("SCOPE_CODE_INVALID")
			},
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/global-scope-packages?scope_code=jobcatalog", nil)
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{
			listGlobalScopePackagesFn: func(context.Context, string) ([]ScopePackage, error) {
				return []ScopePackage{{PackageID: "p1", ScopeCode: "jobcatalog", PackageCode: "PKG1"}}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get lowercase actor header and nil rows", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/global-scope-packages?scope"+"_code=jobcatalog", nil)
		req.Header.Set("x-actor-scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{
			listGlobalScopePackagesFn: func(context.Context, string) ([]ScopePackage, error) {
				return nil, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("post tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"PKG1","name":"Pkg","effective_date":"2026-01-01","request_id":"r1"}`))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/global-scope-packages", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post invalid request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/global-scope-packages", strings.NewReader(`{"scope_code":"","name":"","request_id":""}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post invalid date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"PKG1","name":"Pkg","effective_date":"bad","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post effective date required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"PKG1","name":"Pkg","effective_date":"","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post reserved code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"DEFLT","name":"Pkg","effective_date":"2026-01-01","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post invalid code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"bad-code","name":"Pkg","effective_date":"2026-01-01","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post actor scope required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"PKG1","name":"Pkg","effective_date":"2026-01-01","request_id":"r1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post store error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"PKG1","name":"Pkg","effective_date":"2026-01-01","request_id":"r1"}`))
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{
			createGlobalScopePackageFn: func(context.Context, string, string, string, string, string, string, string) (ScopePackage, error) {
				return ScopePackage{}, errors.New("PACKAGE_CODE_DUPLICATE")
			},
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"PKG1","name":"Pkg","effective_date":"2026-01-01","request_id":"r1"}`))
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{
			createGlobalScopePackageFn: func(context.Context, string, string, string, string, string, string, string) (ScopePackage, error) {
				return ScopePackage{PackageID: "p1", ScopeCode: "jobcatalog", PackageCode: "PKG1", Status: "active"}, nil
			},
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post generate code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/global-scope-packages", strings.NewReader(`{"scope_code":"jobcatalog","package_code":"","name":"Pkg","effective_date":"2026-01-01","request_id":"r1"}`))
		req.Header.Set("X-Actor-Scope", "saas")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{
			createGlobalScopePackageFn: func(_ context.Context, _ string, packageCode string, _ string, _ string, _ string, _ string, _ string) (ScopePackage, error) {
				if !strings.HasPrefix(packageCode, "PKG_") {
					t.Fatalf("unexpected code=%q", packageCode)
				}
				if !packageCodePattern.MatchString(packageCode) {
					t.Fatalf("invalid code=%q", packageCode)
				}
				return ScopePackage{PackageID: "p1", ScopeCode: "jobcatalog", PackageCode: packageCode, Status: "active"}, nil
			},
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandleGlobalScopePackagesAPI_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/org/api/global-scope-packages", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handleGlobalScopePackagesAPI(rec, req, scopeAPIStore{})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestWriteScopeAPIError_StatusMapping(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/scope-packages", nil)

	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "not found", err: errors.New("PACKAGE_NOT_FOUND"), status: http.StatusNotFound},
		{name: "conflict", err: errors.New("PACKAGE_CODE_DUPLICATE"), status: http.StatusConflict},
		{name: "unprocessable", err: errors.New("PACKAGE_CODE_RESERVED"), status: http.StatusUnprocessableEntity},
		{name: "stable", err: errors.New("CODE_123"), status: http.StatusUnprocessableEntity},
		{name: "bad request", err: newBadRequestError("bad"), status: http.StatusBadRequest},
		{name: "pg invalid", err: &pgconn.PgError{Code: "22P02", Message: "invalid"}, status: http.StatusBadRequest},
		{name: "default", err: errors.New(""), status: http.StatusInternalServerError},
	} {
		rec := httptest.NewRecorder()
		writeScopeAPIError(rec, req, tc.err, "fallback")
		if rec.Code != tc.status {
			t.Fatalf("%s status=%d", tc.name, rec.Code)
		}
		if tc.name == "default" && !strings.Contains(rec.Body.String(), "fallback") {
			t.Fatalf("missing default code: %q", rec.Body.String())
		}
	}
}

func TestScopeAPIHelpers(t *testing.T) {
	if got := parseScopePackageID("/org/api/scope-packages/p1/disable"); got != "p1" {
		t.Fatalf("got=%q", got)
	}
	if got := parseScopePackageID("/org/api/scope-packages"); got != "" {
		t.Fatalf("expected empty, got=%q", got)
	}

	code := generatePackageCode()
	if !strings.HasPrefix(code, "PKG_") {
		t.Fatalf("unexpected code=%q", code)
	}
	if ok, _ := regexp.MatchString(`^PKG_[A-Z0-9]{1,8}$`, code); !ok {
		t.Fatalf("unexpected code=%q", code)
	}
}

func TestEnforceSetIDWriteContext(t *testing.T) {
	newReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/org/api/scope-packages", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		return req
	}

	t.Run("business unit required", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ok := enforceSetIDWriteContext(rec, newReq(), scopeAPIStore{}, "t1", "", "2026-01-01", "A0001", "")
		if ok {
			t.Fatalf("expected false")
		}
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), scopeReasonOwnerContextRequired) {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("business unit invalid", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ok := enforceSetIDWriteContext(rec, newReq(), scopeAPIStore{}, "t1", "bad", "2026-01-01", "A0001", "")
		if ok {
			t.Fatalf("expected false")
		}
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), scopeReasonOwnerContextRequired) {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("resolve setid failed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		store := scopeAPIStore{
			resolveSetIDFn: func(context.Context, string, string, string) (string, error) {
				return "", errors.New("SETID_NOT_FOUND")
			},
		}
		ok := enforceSetIDWriteContext(rec, newReq(), store, "t1", "10000001", "2026-01-01", "A0001", "")
		if ok {
			t.Fatalf("expected false")
		}
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), scopeReasonOwnerContextForbidden) {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("resolved setid empty", func(t *testing.T) {
		rec := httptest.NewRecorder()
		store := scopeAPIStore{
			resolveSetIDFn: func(context.Context, string, string, string) (string, error) {
				return "", nil
			},
		}
		ok := enforceSetIDWriteContext(rec, newReq(), store, "t1", "10000001", "2026-01-01", "A0001", "")
		if ok {
			t.Fatalf("expected false")
		}
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), scopeReasonOwnerContextForbidden) {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("owner setid mismatch", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ok := enforceSetIDWriteContext(rec, newReq(), scopeAPIStore{}, "t1", "10000001", "2026-01-01", "B0001", "")
		if ok {
			t.Fatalf("expected false")
		}
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), scopeReasonOwnerContextForbidden) {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("setid mismatch", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ok := enforceSetIDWriteContext(rec, newReq(), scopeAPIStore{}, "t1", "10000001", "2026-01-01", "", "B0001")
		if ok {
			t.Fatalf("expected false")
		}
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), scopeReasonOwnerContextForbidden) {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ok := enforceSetIDWriteContext(rec, newReq(), scopeAPIStore{}, "t1", "10000001", "2026-01-01", "A0001", "")
		if !ok {
			t.Fatalf("expected true")
		}
	})
}
